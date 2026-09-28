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
	benchmarkSearch(b, Options{})
}

func BenchmarkSearchMapped(b *testing.B) {
	benchmarkSearch(b, Options{MapSpans: true})
}

func BenchmarkSearchMappedMiss(b *testing.B) {
	benchmarkSearchQueryWithOptions(b, benchFixture(10*1024*1024), "غيرموجود", false, Options{MapSpans: true})
}

func BenchmarkSearchMappedSparseHit(b *testing.B) {
	var lines strings.Builder
	for lines.Len() < 10*1024*1024 {
		for i := 0; i < 999; i++ {
			lines.WriteString(benchLine)
			lines.WriteByte('\n')
		}
		lines.WriteString("هذا غيرموجود هنا\n")
	}
	benchmarkSearchQueryWithOptions(b, lines.String(), "غيرموجود", true, Options{MapSpans: true})
}

func BenchmarkSearchWordMiss(b *testing.B) {
	benchmarkSearchQueryWithOptions(b, benchFixture(10*1024*1024), "غيرموجود", false, Options{WordRegexp: true})
}

func BenchmarkSearchPlain(b *testing.B) {
	line := "أعلنت المدينة افتتاح مكتبة جديدة.\n"
	var fixture strings.Builder
	for fixture.Len() < 10*1024*1024 {
		fixture.WriteString(line)
	}
	benchmarkSearchQuery(b, fixture.String(), "مكتبة", true)
}

func BenchmarkSearchPlainSummary(b *testing.B) {
	line := "أعلنت المدينة افتتاح مكتبة جديدة.\n"
	var fixture strings.Builder
	for fixture.Len() < 10*1024*1024 {
		fixture.WriteString(line)
	}
	benchmarkSearchQueryWithOptions(b, fixture.String(), "مكتبة", true, Options{OmitText: true})
}

func BenchmarkSearchMiss(b *testing.B) {
	benchmarkSearchQuery(b, benchFixture(10*1024*1024), "غيرموجود", false)
}

func BenchmarkSearchSparseHit(b *testing.B) {
	var lines strings.Builder
	for lines.Len() < 10*1024*1024 {
		for i := 0; i < 999; i++ {
			lines.WriteString(benchLine)
			lines.WriteByte('\n')
		}
		lines.WriteString("هذا غيرموجود هنا\n")
	}
	benchmarkSearchQuery(b, lines.String(), "غيرموجود", true)
}

func benchmarkSearchQuery(b *testing.B, fixture, query string, wantFound bool) {
	benchmarkSearchQueryWithOptions(b, fixture, query, wantFound, Options{})
}

func benchmarkSearchQueryWithOptions(b *testing.B, fixture, query string, wantFound bool, opts Options) {
	m := mustLiteral(b, query)
	b.SetBytes(int64(len(fixture)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, err := Search(strings.NewReader(fixture), m, opts, func(Match) error { return nil })
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if found != wantFound {
			b.Fatalf("Search found=%v; want %v", found, wantFound)
		}
	}
}

func benchmarkSearch(b *testing.B, opts Options) {
	const fixtureSize = 10 * 1024 * 1024
	fixture := benchFixture(fixtureSize)

	m := mustLiteral(b, "الطلاب")

	b.SetBytes(int64(len(fixture)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, err := Search(strings.NewReader(fixture), m, opts, func(Match) error { return nil })
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if !found {
			b.Fatal("expected at least one match")
		}
	}
}
