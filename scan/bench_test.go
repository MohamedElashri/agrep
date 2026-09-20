package scan

import (
	"strings"
	"testing"
)

// benchLine is one fully-voweled logical line, repeated to build fixtures.
// It carries tashkil, tatweel, and a seated-hamza variant so normalization
// does real work on every line rather than short-circuiting.
const benchLine = "هَذِهِ مَدْرَسَةٌ " +
	"العــربية مسؤول عن " +
	"الطَّلابِ"

// benchFixture builds a newline-delimited fixture of approximately n bytes
// by repeating benchLine. It is generated rather than checked into the repo
// so the benchmark has no test-data maintenance cost.
func benchFixture(n int) string {
	var b strings.Builder
	b.Grow(n + len(benchLine) + 1)
	for b.Len() < n {
		b.WriteString(benchLine)
		b.WriteByte('\n')
	}
	return b.String()
}

func BenchmarkSearch(b *testing.B) {
	const fixtureSize = 10 * 1024 * 1024
	fixture := benchFixture(fixtureSize)

	m := mustLiteral(b, "الطلاب")

	b.SetBytes(int64(len(fixture)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, err := Search(strings.NewReader(fixture), m, Options{}, func(Match) error { return nil })
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if !found {
			b.Fatal("expected at least one match")
		}
	}
}
