package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/MohamedElashri/agrep/scan"
)

func jsonEmitter(w io.Writer) func(scan.Match) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return func(m scan.Match) error {
		if m.File == "(standard input)" {
			m.File = ""
		}
		return encoder.Encode(m)
	}
}

func humanEmitter(w io.Writer, lineNumbers, filenames bool) func(scan.Match) error {
	return func(m scan.Match) error {
		if m.GroupStart {
			if _, err := fmt.Fprintln(w, "--"); err != nil {
				return err
			}
		}
		separator := ":"
		if m.Context {
			separator = "-"
		}
		var prefix strings.Builder
		if filenames {
			prefix.WriteString(m.File)
			prefix.WriteString(separator)
		}
		if lineNumbers {
			prefix.WriteString(strconv.FormatInt(m.Line, 10))
			prefix.WriteString(separator)
		}
		_, err := fmt.Fprintln(w, prefix.String()+m.Text)
		return err
	}
}
