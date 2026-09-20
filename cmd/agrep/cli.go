package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/MohamedElashri/agrep/arabic"
	"github.com/MohamedElashri/agrep/match"
	"github.com/MohamedElashri/agrep/scan"
)

const usageText = `Usage: agrep [options] <query> [file]

Search Arabic text while ignoring tashkil, tatweel, canonical Unicode form,
and common Alef, Hamza, Ta-Marbuta, and Alef-Maksura spelling differences.
Use file "-" or omit file to read standard input.

Options:
  -j, --json                     emit one JSON object per matching line
  -n, --line-number              prefix human output with the 1-based line number
      --max-line-bytes N         reject longer logical lines (0 means unlimited)
      --profile NAME             search|strict|loose|lucene|camel (default: search)
      --keep-hamza                don't fold hamza/madda variants (أ إ آ ٱ ؤ ئ)
      --keep-tamarbuta             don't fold ة to ه
      --keep-tashkil               don't strip tashkil (diacritics)
      --keep-presentation-forms    don't expand ligatures/contextual letter forms
      --keep-joiners               don't strip ZWNJ/ZWJ
      --keep-bidi-marks            don't strip bidi control characters
      --keep-quranic-marks         don't strip Quranic annotation marks
      --fold-digits                fold Arabic-Indic/Extended Arabic-Indic digits to ASCII
      --fold-punctuation           fold Arabic punctuation to ASCII
      --version                    print version and exit
  -h, --help                       show this help

Profiles:
  search  agrep's original, default behavior: every fold enabled except
          digit/punctuation folding.
  strict  strips only cosmetic marks (tashkil, tatweel, presentation forms,
          joiners, bidi marks, Quranic marks); keeps every letter-level
          distinction (hamza, ta-marbuta, alef-maksura).
  loose   every fold this build defines, including digit/punctuation folding.
  lucene  matches Apache Lucene's ArabicNormalizer.
  camel   matches CAMeL Tools' normalize_alef_ar/dediac_ar family.
See docs/NORMALIZATION.md for exactly what each profile does and why.

--keep-*/--fold-* flags apply on top of --profile.

Exit status: 0 if matched, 1 if not matched, 2 on an error.
`

type cliOptions struct {
	jsonOutput   bool
	lineNumbers  bool
	maxLineBytes uint64
	profileName  string
	overrides    profileOverrides
	showHelp     bool
	showVersion  bool
}

// profileOverrides are the --keep-*/--fold-* flags applied on top of
// whichever preset --profile selects. Each keep* flag only ever clears a
// field the preset may have left on; each fold* flag only ever sets one the
// preset may have left off. None of the nine can conflict with another —
// they touch disjoint fields — so there is no ordering ambiguity to resolve
// between them; see resolveProfile.
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
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, positional, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}

	profile, err := resolveProfile(opts.profileName, opts.overrides)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}

	query := positional[0]
	matcher, err := match.NewLiteral(query, profile)
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}

	input := stdin
	var file *os.File
	if len(positional) == 2 && positional[1] != "-" {
		var err error
		file, err = os.Open(positional[1])
		if err != nil {
			fmt.Fprintf(stderr, "agrep: %v\n", err)
			return 2
		}
		input = file
	}

	emit := humanEmitter(stdout, opts.lineNumbers)
	if opts.jsonOutput {
		emit = jsonEmitter(stdout)
	}

	found, err := scan.Search(input, matcher, scan.Options{MaxLineBytes: opts.maxLineBytes}, emit)
	if file != nil {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "agrep: %v\n", err)
		return 2
	}
	if !found {
		return 1
	}
	return 0
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
	return p, nil
}

func parseArgs(args []string, stdout, stderr io.Writer) (cliOptions, []string, int) {
	var opts cliOptions
	fs := flag.NewFlagSet("agrep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usageText) }

	fs.BoolVar(&opts.jsonOutput, "j", false, "emit JSON Lines")
	fs.BoolVar(&opts.jsonOutput, "json", false, "emit JSON Lines")
	fs.BoolVar(&opts.lineNumbers, "n", false, "print line numbers")
	fs.BoolVar(&opts.lineNumbers, "line-number", false, "print line numbers")
	fs.Uint64Var(&opts.maxLineBytes, "max-line-bytes", 0, "maximum logical line size")
	fs.StringVar(&opts.profileName, "profile", "search", "normalization profile")
	fs.BoolVar(&opts.overrides.keepHamza, "keep-hamza", false, "don't fold hamza/madda variants")
	fs.BoolVar(&opts.overrides.keepTaMarbuta, "keep-tamarbuta", false, "don't fold ta-marbuta to heh")
	fs.BoolVar(&opts.overrides.keepTashkil, "keep-tashkil", false, "don't strip tashkil")
	fs.BoolVar(&opts.overrides.keepPresentationForms, "keep-presentation-forms", false, "don't expand ligatures/contextual letter forms")
	fs.BoolVar(&opts.overrides.keepJoiners, "keep-joiners", false, "don't strip ZWNJ/ZWJ")
	fs.BoolVar(&opts.overrides.keepBidiMarks, "keep-bidi-marks", false, "don't strip bidi control characters")
	fs.BoolVar(&opts.overrides.keepQuranicMarks, "keep-quranic-marks", false, "don't strip Quranic annotation marks")
	fs.BoolVar(&opts.overrides.foldDigits, "fold-digits", false, "fold Arabic-Indic/Extended Arabic-Indic digits to ASCII")
	fs.BoolVar(&opts.overrides.foldPunctuation, "fold-punctuation", false, "fold Arabic punctuation to ASCII")
	fs.BoolVar(&opts.showHelp, "h", false, "show help")
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
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fmt.Fprint(stderr, usageText)
		return opts, nil, 2
	}

	return opts, fs.Args(), -1
}
