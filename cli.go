package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const usageText = `Usage: agrep [options] <query> [file]

Search Arabic text while ignoring tashkil, tatweel, canonical Unicode form,
and common Alef, Hamza, Ta-Marbuta, and Alef-Maksura spelling differences.
Use file "-" or omit file to read standard input.

Options:
  -j, --json                 emit one JSON object per matching line
  -n, --line-number          prefix human output with the 1-based line number
      --max-line-bytes N     reject longer logical lines (0 means unlimited)
      --version              print version and exit
  -h, --help                 show this help

Exit status: 0 if matched, 1 if not matched, 2 on an error.
`

type cliOptions struct {
	jsonOutput   bool
	lineNumbers bool
	maxLineBytes uint64
	showHelp     bool
	showVersion  bool
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, positional, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}

	query := positional[0]
	if _, err := normalizeQuery(query); err != nil {
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

	found, err := search(input, query, searchOptions{MaxLineBytes: opts.maxLineBytes}, emit)
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

func jsonEmitter(w io.Writer) func(Match) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return func(m Match) error { return encoder.Encode(m) }
}

func humanEmitter(w io.Writer, lineNumbers bool) func(Match) error {
	if lineNumbers {
		return func(m Match) error {
			_, err := fmt.Fprintf(w, "%d:%s\n", m.Line, m.Text)
			return err
		}
	}
	return func(m Match) error {
		_, err := fmt.Fprintln(w, m.Text)
		return err
	}
}
