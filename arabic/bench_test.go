package arabic

import "testing"

// benchLine is one fully-voweled logical line. It carries tashkil, tatweel,
// and a seated-hamza variant so normalization does real work rather than
// short-circuiting.
const benchLine = "هَذِهِ مَدْرَسَةٌ " +
	"العــربية مسؤول عن " +
	"الطَّلابِ"

func BenchmarkNormalize(b *testing.B) {
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Normalize(benchLine)
	}
}

func BenchmarkNormalizeRasm(b *testing.B) {
	p := ProfileSearch
	p.Rasm = true
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = p.Normalize(benchLine)
	}
}

func BenchmarkNormalizeMappedRasm(b *testing.B) {
	p := ProfileSearch
	p.Rasm = true
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.NormalizeMapped(benchLine)
	}
}
