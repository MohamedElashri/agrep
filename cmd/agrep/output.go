package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/MohamedElashri/agrep/scan"
)

func jsonEmitter(w io.Writer) func(scan.Match) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return func(m scan.Match) error { return encoder.Encode(m) }
}

func humanEmitter(w io.Writer, lineNumbers bool) func(scan.Match) error {
	if lineNumbers {
		return func(m scan.Match) error {
			_, err := fmt.Fprintf(w, "%d:%s\n", m.Line, m.Text)
			return err
		}
	}
	return func(m scan.Match) error {
		_, err := fmt.Fprintln(w, m.Text)
		return err
	}
}
