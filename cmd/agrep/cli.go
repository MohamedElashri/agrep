package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/internal/decode"
	"github.com/MohamedElashri/agrep/internal/inputformat"
	"github.com/MohamedElashri/agrep/internal/translit"
	"github.com/MohamedElashri/agrep/internal/walk"
	"github.com/MohamedElashri/agrep/match"
	"github.com/MohamedElashri/agrep/scan"
)

const usageText = `Usage: agrep [options] <query> [path ...]
       agrep [options] -e <query>... [path ...]

Search Arabic text while ignoring tashkil, tatweel, canonical Unicode form,
and common Alef, Hamza, Ta-Marbuta, and Alef-Maksura spelling differences.
With no path, read standard input. Use path "-" to read standard input among
other inputs.

Search options:
  -e PATTERN                       add a pattern (repeatable)
  -i, --ignore-case               apply Unicode case folding
  -v, --invert-match              select non-matching lines
  -w, --word-regexp               require original-text word boundaries
      --regex                     treat patterns as regular expressions over normalized text
      --fuzzy[=N]                 allow N Levenshtein edits (default N: 1)
  -r, --recursive                 walk directories (or the current directory)
      --include GLOB              search only matching files (repeatable)
      --exclude GLOB              skip matching files/directories (repeatable)
      --no-ignore                 do not honor .gitignore files
      --threads N                 file-search workers (default: available CPUs)
      --encoding NAME             utf8|cp1256|iso-8859-6|utf16le|utf16be|auto
      --translit NAME             buckwalter|arabtex|iso233 query input

Output options:
  -j, --json                      emit one JSON object per selected/context line
  -n, --line-number               prefix human output with the 1-based line number
  -c, --count                     print selected-line counts
  -l, --files-with-matches        print files containing selected lines
  -L, --files-without-match       print files containing no selected lines
  -A N, --after-context N         print N lines after selected lines
  -B N, --before-context N        print N lines before selected lines
  -C N, --context N               print N lines before and after selected lines
  -H, --with-filename             always print filename prefixes
  -h, --no-filename               never print filename prefixes
  -o, --only-matching             print only original-text matched spans
      --color WHEN                auto|always|never (default: auto)
      --no-bidi-isolate           omit RTL isolates around colored Arabic spans
      --translit-out              render emitted text as Buckwalter

Normalization options:
      --max-line-bytes N          reject longer logical lines (0 means unlimited)
      --profile NAME              search|strict|loose|lucene|camel (default: search)
      --lang LIST                 ar,fa,ur,ps,ku,ug (default: ar)
      --rasm                      fold Arabic consonants to dotless skeletons
      --keep-hamza                don't fold hamza/madda variants (أ إ آ ٱ ؤ ئ)
      --keep-tamarbuta            don't fold ة to ه
      --keep-tashkil              don't strip tashkil (diacritics)
      --keep-presentation-forms   don't expand ligatures/contextual letter forms
      --keep-joiners              don't strip ZWNJ/ZWJ
      --keep-bidi-marks           don't strip bidi control characters
      --keep-quranic-marks        don't strip Quranic annotation marks
      --fold-digits               fold Arabic-Indic/Extended Arabic-Indic digits to ASCII
      --fold-punctuation          fold Arabic punctuation to ASCII
      --version                   print version and exit
      --help                      show this help

Profiles:
  search  agrep's original, default behavior: every fold enabled except
          digit/punctuation folding.
  strict  strips only cosmetic marks; keeps every letter-level distinction.
  loose   every fold this build defines, including digit/punctuation and rasm.
  lucene  matches Apache Lucene's ArabicNormalizer.
  camel   matches CAMeL Tools' normalize_alef_ar/dediac_ar family.
See docs/NORMALIZATION.md for exactly what each profile does and why.

--keep-*/--fold-* flags apply on top of --profile.

Exit status: 0 if selected, 1 if nothing was selected, 2 on an error.
`

type filenameMode uint8

const (
	filenameAuto filenameMode = iota
	filenameAlways
	filenameNever
)

type colorMode uint8

const (
	colorAuto colorMode = iota
	colorAlways
	colorNever
)

func (m *colorMode) String() string {
	switch *m {
	case colorAlways:
		return "always"
	case colorNever:
		return "never"
	default:
		return "auto"
	}
}

func (m *colorMode) Set(value string) error {
	switch value {
	case "auto":
		*m = colorAuto
	case "always":
		*m = colorAlways
	case "never":
		*m = colorNever
	default:
		return fmt.Errorf("color must be auto, always, or never")
	}
	return nil
}

func colorEnabled(mode colorMode, output io.Writer) bool {
	switch mode {
	case colorAlways:
		return true
	case colorNever:
		return false
	}
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type cliOptions struct {
	jsonOutput        bool
	lineNumbers       bool
	maxLineBytes      uint64
	profileName       string
	languageNames     string
	overrides         profileOverrides
	showHelp          bool
	showVersion       bool
	recursive         bool
	ignoreCase        bool
	invertMatch       bool
	wordRegexp        bool
	regex             bool
	fuzzy             fuzzyFlag
	countOnly         bool
	filesWithMatches  bool
	filesWithoutMatch bool
	beforeContext     int
	afterContext      int
	filenameMode      filenameMode
	patterns          stringList
	includes          stringList
	excludes          stringList
	noIgnore          bool
	threads           int
	onlyMatching      bool
	color             colorMode
	noBidiIsolate     bool
	encodingName      string
	inputEncoding     decode.Encoding
	translitName      string
	translitScheme    translit.Scheme
	translitOut       bool
}

// profileOverrides are the --keep-*/--fold-* flags applied on top of
// whichever preset --profile selects.
type profileOverrides struct {
	keepHamza             bool
	keepTaMarbuta         bool
	keepTashkil           bool
	keepPresentationForms bool
	keepJoiners           bool
	keepBidiMarks         bool
	keepQuranicMarks      bool
	foldDigits            bool
	foldPunctuation       bool
	rasm                  bool
}

type inputSpec struct {
	path  string
	label string
	stdin bool
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, positional, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}

	queries, paths, err := resolveOperands(opts.patterns, positional)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}
	opts.inputEncoding, err = decode.Parse(opts.encodingName)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}
	if opts.translitName != "" {
		opts.translitScheme, err = translit.Parse(opts.translitName)
		if err != nil {
			fmt.Fprintf(stderr, "agrep: %v\n", err)
			return 2
		}
		for i := range queries {
			queries[i] = translit.FromLatin(queries[i], opts.translitScheme)
		}
	}
	profile, err := resolveProfile(opts.profileName, opts.overrides)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}
	profile.Languages, err = arabic.ParseLanguages(opts.languageNames)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}
	var matcher match.Matcher
	if opts.regex {
		matcher, err = match.NewRegex(queries, profile, opts.ignoreCase)
	} else if opts.fuzzy.enabled {
		matcher, err = match.NewFuzzy(queries, profile, opts.ignoreCase, opts.fuzzy.distance)
	} else {
		matcher, err = match.NewLiterals(queries, profile, opts.ignoreCase)
	}
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}

	inputs, discoverErr := discoverInputs(paths, opts)
	if len(inputs) == 0 && len(paths) == 0 && !opts.recursive {
		inputs = []inputSpec{{label: "(standard input)", stdin: true}}
	}
	showFilenames := opts.filenameMode == filenameAlways ||
		(opts.filenameMode == filenameAuto && (len(inputs) > 1 || opts.recursive))
	highlight := colorEnabled(opts.color, stdout)

	hadSelection, searchErr := searchInputs(inputs, stdin, stdout, matcher, opts, showFilenames, highlight)
	err = errors.Join(discoverErr, searchErr)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}
	if !hadSelection {
		return 1
	}
	return 0
}

func resolveOperands(patterns []string, positional []string) (queries, paths []string, err error) {
	if len(patterns) > 0 {
		return append([]string(nil), patterns...), positional, nil
	}
	if len(positional) == 0 {
		return nil, nil, errors.New("missing query")
	}
	return []string{positional[0]}, positional[1:], nil
}

func discoverInputs(paths []string, opts cliOptions) ([]inputSpec, error) {
	if len(paths) == 0 && opts.recursive {
		paths = []string{"."}
	}
	seen := make(map[string]struct{})
	stdinSeen := false
	var inputs []inputSpec
	var errs []error
	for _, name := range paths {
		if name == "-" {
			if !stdinSeen {
				inputs = append(inputs, inputSpec{label: "(standard input)", stdin: true})
				stdinSeen = true
			}
			continue
		}
		files, err := walk.Collect([]string{name}, walk.Options{
			Recursive: opts.recursive,
			NoIgnore:  opts.noIgnore,
			Includes:  opts.includes,
			Excludes:  opts.excludes,
		})
		if err != nil {
			errs = append(errs, err)
		}
		for _, file := range files {
			if _, ok := seen[file]; ok {
				continue
			}
			seen[file] = struct{}{}
			inputs = append(inputs, inputSpec{path: file, label: file})
		}
	}
	return inputs, errors.Join(errs...)
}

type fileResult struct {
	index     int
	output    []byte
	found     bool
	qualifies bool
	err       error
}

func searchInputs(inputs []inputSpec, stdin io.Reader, stdout io.Writer, matcher match.Matcher, opts cliOptions, showFilenames, highlight bool) (bool, error) {
	if len(inputs) == 1 {
		found, qualifies, err := searchOne(inputs[0], stdin, stdout, matcher, opts, showFilenames, highlight)
		if opts.filesWithoutMatch {
			return qualifies, err
		}
		return found, err
	}
	if len(inputs) == 0 {
		return false, nil
	}

	workers := opts.threads
	if workers > len(inputs) {
		workers = len(inputs)
	}
	jobs := make(chan int)
	results := make(chan fileResult, len(inputs))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				var buffer bytes.Buffer
				found, qualifies, err := searchOne(inputs[index], stdin, &buffer, matcher, opts, showFilenames, highlight)
				results <- fileResult{index: index, output: buffer.Bytes(), found: found, qualifies: qualifies, err: err}
			}
		}()
	}
	go func() {
		for index := range inputs {
			jobs <- index
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	pending := make(map[int]fileResult)
	next := 0
	selected := false
	var errs []error
	var writeErr error
	for result := range results {
		pending[result.index] = result
		for {
			ordered, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			if opts.filesWithoutMatch {
				selected = selected || ordered.qualifies
			} else {
				selected = selected || ordered.found
			}
			if ordered.err != nil {
				errs = append(errs, ordered.err)
			}
			if writeErr == nil && len(ordered.output) > 0 {
				_, writeErr = stdout.Write(ordered.output)
			}
			next++
		}
	}
	return selected, errors.Join(append(errs, writeErr)...)
}

func searchOne(input inputSpec, stdin io.Reader, output io.Writer, matcher match.Matcher, opts cliOptions, showFilenames, highlight bool) (found, qualifies bool, err error) {
	reader := stdin
	selectedEncoding := opts.inputEncoding
	var file io.ReadCloser
	if !input.stdin {
		var extractedUTF8 bool
		file, extractedUTF8, err = inputformat.Open(input.path)
		if err != nil {
			return false, false, fmt.Errorf("%s: %w", input.path, err)
		}
		if extractedUTF8 {
			if opts.inputEncoding != decode.UTF8 && opts.inputEncoding != decode.Auto {
				_ = file.Close()
				return false, false, fmt.Errorf("%s: extracted document text is UTF-8 and cannot use --encoding=%s", input.path, opts.inputEncoding)
			}
			selectedEncoding = decode.UTF8
		}
		reader = file
		defer func() {
			if closeErr := file.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
		}()
	}
	reader, _, err = decode.NewReader(reader, selectedEncoding)
	if err != nil {
		return false, false, fmt.Errorf("%s: %w", input.label, err)
	}

	contextLines := !opts.countOnly && !opts.filesWithMatches && !opts.filesWithoutMatch
	searchOpts := scan.Options{
		MaxLineBytes: opts.maxLineBytes,
		File:         input.label,
		InvertMatch:  opts.invertMatch,
		WordRegexp:   opts.wordRegexp,
		MapSpans:     contextLines && (opts.jsonOutput || opts.onlyMatching || highlight),
		OmitText:     !contextLines,
	}
	if contextLines {
		searchOpts.BeforeContext = opts.beforeContext
		searchOpts.AfterContext = opts.afterContext
	}

	count := int64(0)
	emit := humanEmitter(output, humanOutputOptions{
		lineNumbers:  opts.lineNumbers,
		filenames:    showFilenames,
		onlyMatching: opts.onlyMatching,
		highlight:    highlight,
		bidiIsolate:  !opts.noBidiIsolate,
	})
	if opts.jsonOutput {
		emit = jsonEmitter(output)
	}
	if opts.translitOut {
		next := emit
		emit = func(mt scan.Match) error {
			return next(transliterateMatch(mt))
		}
	}
	found, err = scan.Search(reader, matcher, searchOpts, func(mt scan.Match) error {
		if !mt.Context {
			count++
		}
		if contextLines {
			return emit(mt)
		}
		return nil
	})
	if err != nil {
		if selectedEncoding == decode.UTF8 && errors.Is(err, scan.ErrInvalidUTF8) {
			return false, false, fmt.Errorf("%s: %w; try --encoding=auto", input.label, err)
		}
		return false, false, fmt.Errorf("%s: %w", input.label, err)
	}

	switch {
	case opts.countOnly:
		if showFilenames {
			_, err = fmt.Fprintf(output, "%s:%d\n", input.label, count)
		} else {
			_, err = fmt.Fprintln(output, count)
		}
	case opts.filesWithMatches && found:
		_, err = fmt.Fprintln(output, input.label)
	case opts.filesWithoutMatch && !found:
		_, err = fmt.Fprintln(output, input.label)
	}
	return found, !found, err
}

func transliterateMatch(mt scan.Match) scan.Match {
	rendered, boundaries := translit.ToLatin(mt.Text, translit.Buckwalter)
	spans := make([]scan.Span, 0, len(mt.Spans))
	for _, span := range mt.Spans {
		start, end, ok := translit.MapSpan(boundaries, span[0], span[1])
		if ok {
			spans = append(spans, scan.Span{start, end})
		}
	}
	mt.Text = rendered
	mt.Spans = spans
	return mt
}

// resolveProfile selects a named preset and then applies o on top of it.
func resolveProfile(name string, o profileOverrides) (arabic.Profile, error) {
	var p arabic.Profile
	switch name {
	case "", "search":
		p = arabic.ProfileSearch
	case "strict":
		p = arabic.ProfileStrict
	case "loose":
		p = arabic.ProfileLoose
	case "lucene":
		p = arabic.ProfileLucene
	case "camel":
		p = arabic.ProfileCAMeL
	default:
		return arabic.Profile{}, fmt.Errorf("unknown --profile %q (want search, strict, loose, lucene, or camel)", name)
	}

	if o.keepHamza {
		p.FoldAlefHamza = false
		p.FoldAlefWasla = false
		p.FoldHamzaSeat = false
	}
	if o.keepTaMarbuta {
		p.FoldTaMarbuta = false
	}
	if o.keepTashkil {
		p.StripTashkil = false
	}
	if o.keepPresentationForms {
		p.FoldPresentation = false
	}
	if o.keepJoiners {
		p.StripJoiners = false
	}
	if o.keepBidiMarks {
		p.StripBidi = false
	}
	if o.keepQuranicMarks {
		p.StripQuranic = false
	}
	if o.foldDigits {
		p.FoldDigits = true
	}
	if o.foldPunctuation {
		p.FoldPunctuation = true
	}
	if o.rasm {
		p.Rasm = true
	}
	return p, nil
}

func parseArgs(args []string, stdout, stderr io.Writer) (cliOptions, []string, int) {
	opts := cliOptions{threads: runtime.GOMAXPROCS(0)}
	fs := flag.NewFlagSet("agrep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usageText) }

	fs.BoolVar(&opts.jsonOutput, "j", false, "emit JSON Lines")
	fs.BoolVar(&opts.jsonOutput, "json", false, "emit JSON Lines")
	fs.BoolVar(&opts.lineNumbers, "n", false, "print line numbers")
	fs.BoolVar(&opts.lineNumbers, "line-number", false, "print line numbers")
	fs.BoolVar(&opts.recursive, "r", false, "walk directories")
	fs.BoolVar(&opts.recursive, "recursive", false, "walk directories")
	fs.BoolVar(&opts.ignoreCase, "i", false, "Unicode case folding")
	fs.BoolVar(&opts.ignoreCase, "ignore-case", false, "Unicode case folding")
	fs.BoolVar(&opts.invertMatch, "v", false, "select non-matching lines")
	fs.BoolVar(&opts.invertMatch, "invert-match", false, "select non-matching lines")
	fs.BoolVar(&opts.wordRegexp, "w", false, "require original-text word boundaries")
	fs.BoolVar(&opts.wordRegexp, "word-regexp", false, "require original-text word boundaries")
	fs.BoolVar(&opts.regex, "regex", false, "regular expressions over normalized text")
	fs.Var(&opts.fuzzy, "fuzzy", "allow Levenshtein edits")
	fs.BoolVar(&opts.countOnly, "c", false, "print match counts")
	fs.BoolVar(&opts.countOnly, "count", false, "print match counts")
	fs.BoolVar(&opts.filesWithMatches, "l", false, "print files with matches")
	fs.BoolVar(&opts.filesWithMatches, "files-with-matches", false, "print files with matches")
	fs.BoolVar(&opts.filesWithoutMatch, "L", false, "print files without matches")
	fs.BoolVar(&opts.filesWithoutMatch, "files-without-match", false, "print files without matches")
	fs.Var(&opts.patterns, "e", "pattern")
	fs.Var(&opts.includes, "include", "include glob")
	fs.Var(&opts.excludes, "exclude", "exclude glob")
	fs.BoolVar(&opts.noIgnore, "no-ignore", false, "disable .gitignore handling")
	fs.IntVar(&opts.threads, "threads", opts.threads, "file-search workers")
	fs.Var(contextFlag{before: &opts.beforeContext}, "B", "lines before matches")
	fs.Var(contextFlag{before: &opts.beforeContext}, "before-context", "lines before matches")
	fs.Var(contextFlag{after: &opts.afterContext}, "A", "lines after matches")
	fs.Var(contextFlag{after: &opts.afterContext}, "after-context", "lines after matches")
	fs.Var(contextFlag{before: &opts.beforeContext, after: &opts.afterContext}, "C", "lines around matches")
	fs.Var(contextFlag{before: &opts.beforeContext, after: &opts.afterContext}, "context", "lines around matches")
	fs.Var(filenameFlag{mode: &opts.filenameMode, value: filenameAlways}, "H", "print filenames")
	fs.Var(filenameFlag{mode: &opts.filenameMode, value: filenameAlways}, "with-filename", "print filenames")
	fs.Var(filenameFlag{mode: &opts.filenameMode, value: filenameNever}, "h", "suppress filenames")
	fs.Var(filenameFlag{mode: &opts.filenameMode, value: filenameNever}, "no-filename", "suppress filenames")
	fs.BoolVar(&opts.onlyMatching, "o", false, "print only matched spans")
	fs.BoolVar(&opts.onlyMatching, "only-matching", false, "print only matched spans")
	fs.Var(&opts.color, "color", "auto, always, or never")
	fs.BoolVar(&opts.noBidiIsolate, "no-bidi-isolate", false, "omit RTL isolates around colored spans")
	fs.StringVar(&opts.encodingName, "encoding", "utf8", "input encoding")
	fs.StringVar(&opts.translitName, "translit", "", "transliterated query scheme")
	fs.BoolVar(&opts.translitOut, "translit-out", false, "render output as Buckwalter")
	fs.Uint64Var(&opts.maxLineBytes, "max-line-bytes", 0, "maximum logical line size")
	fs.StringVar(&opts.profileName, "profile", "search", "normalization profile")
	fs.StringVar(&opts.languageNames, "lang", "ar", "Arabic-script languages")
	fs.BoolVar(&opts.overrides.rasm, "rasm", false, "fold Arabic consonants to dotless skeletons")
	fs.BoolVar(&opts.overrides.keepHamza, "keep-hamza", false, "don't fold hamza/madda variants")
	fs.BoolVar(&opts.overrides.keepTaMarbuta, "keep-tamarbuta", false, "don't fold ta-marbuta to heh")
	fs.BoolVar(&opts.overrides.keepTashkil, "keep-tashkil", false, "don't strip tashkil")
	fs.BoolVar(&opts.overrides.keepPresentationForms, "keep-presentation-forms", false, "don't expand ligatures/contextual letter forms")
	fs.BoolVar(&opts.overrides.keepJoiners, "keep-joiners", false, "don't strip ZWNJ/ZWJ")
	fs.BoolVar(&opts.overrides.keepBidiMarks, "keep-bidi-marks", false, "don't strip bidi control characters")
	fs.BoolVar(&opts.overrides.keepQuranicMarks, "keep-quranic-marks", false, "don't strip Quranic annotation marks")
	fs.BoolVar(&opts.overrides.foldDigits, "fold-digits", false, "fold Arabic-Indic/Extended Arabic-Indic digits to ASCII")
	fs.BoolVar(&opts.overrides.foldPunctuation, "fold-punctuation", false, "fold Arabic punctuation to ASCII")
	fs.BoolVar(&opts.showHelp, "help", false, "show help")
	fs.BoolVar(&opts.showVersion, "version", false, "print version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return opts, nil, 0
		}
		return opts, nil, 2
	}
	if opts.showHelp {
		fmt.Fprint(stdout, usageText)
		return opts, nil, 0
	}
	if opts.showVersion {
		fmt.Fprintf(stdout, "agrep %s\n", version)
		return opts, nil, 0
	}
	if opts.threads < 1 {
		fmt.Fprintln(stderr, "agrep: --threads must be at least 1")
		return opts, nil, 2
	}
	modes := 0
	for _, enabled := range []bool{opts.countOnly, opts.filesWithMatches, opts.filesWithoutMatch} {
		if enabled {
			modes++
		}
	}
	if modes > 1 {
		fmt.Fprintln(stderr, "agrep: --count, --files-with-matches, and --files-without-match are mutually exclusive")
		return opts, nil, 2
	}
	if opts.jsonOutput && modes > 0 {
		fmt.Fprintln(stderr, "agrep: --json cannot be combined with count or filename-only output")
		return opts, nil, 2
	}
	if opts.onlyMatching && (opts.invertMatch || modes > 0 || opts.beforeContext > 0 || opts.afterContext > 0) {
		fmt.Fprintln(stderr, "agrep: --only-matching cannot be combined with invert, summary, or context modes")
		return opts, nil, 2
	}
	if opts.regex && opts.translitName != "" {
		fmt.Fprintln(stderr, "agrep: --translit cannot be combined with --regex")
		return opts, nil, 2
	}
	if opts.regex && opts.fuzzy.enabled {
		fmt.Fprintln(stderr, "agrep: --regex and --fuzzy are mutually exclusive")
		return opts, nil, 2
	}
	return opts, fs.Args(), -1
}

type fuzzyFlag struct {
	enabled  bool
	distance int
}

func (f *fuzzyFlag) String() string {
	if !f.enabled {
		return "false"
	}
	return strconv.Itoa(f.distance)
}

func (f *fuzzyFlag) IsBoolFlag() bool { return true }

func (f *fuzzyFlag) Set(value string) error {
	switch value {
	case "true":
		f.enabled = true
		f.distance = 1
		return nil
	case "false":
		f.enabled = false
		f.distance = 0
		return nil
	}
	distance, err := strconv.Atoi(value)
	if err != nil || distance < 0 {
		return errors.New("fuzzy distance must be a non-negative integer")
	}
	f.enabled = true
	f.distance = distance
	return nil
}

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type contextFlag struct {
	before *int
	after  *int
}

func (f contextFlag) String() string { return "0" }
func (f contextFlag) Set(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return errors.New("context must be a non-negative integer")
	}
	if f.before != nil {
		*f.before = n
	}
	if f.after != nil {
		*f.after = n
	}
	return nil
}

type filenameFlag struct {
	mode  *filenameMode
	value filenameMode
}

func (f filenameFlag) String() string   { return "false" }
func (f filenameFlag) IsBoolFlag() bool { return true }
func (f filenameFlag) Set(value string) error {
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	if enabled {
		*f.mode = f.value
	} else {
		*f.mode = filenameAuto
	}
	return nil
}
