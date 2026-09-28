package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type corpusMatch struct {
	Line  int      `json:"line"`
	Text  string   `json:"text"`
	Spans [][2]int `json:"spans"`
}

type corpusCase struct {
	Name    string        `json:"name"`
	File    string        `json:"file"`
	Args    []string      `json:"args"`
	Exit    int           `json:"exit"`
	Matches []corpusMatch `json:"matches"`
}

func TestConformanceCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "corpus")
	data, err := os.ReadFile(filepath.Join(root, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []corpusCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("conformance corpus has no cases")
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			file := filepath.Join(root, tc.File)
			args := append([]string{"--json"}, tc.Args...)
			args = append(args, file)
			var stdout, stderr bytes.Buffer
			if code := run(args, nil, &stdout, &stderr); code != tc.Exit {
				t.Fatalf("exit=%d, want=%d; stderr=%q", code, tc.Exit, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
			var got []corpusMatch
			decoder := json.NewDecoder(&stdout)
			for {
				var record struct {
					File string `json:"file"`
					corpusMatch
				}
				if err := decoder.Decode(&record); err == io.EOF {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				if record.File != file {
					t.Fatalf("file=%q, want %q", record.File, file)
				}
				got = append(got, record.corpusMatch)
			}
			if len(got) == 0 && len(tc.Matches) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.Matches) {
				t.Fatalf("matches=%#v, want %#v", got, tc.Matches)
			}
		})
	}
}
