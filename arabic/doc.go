// Package arabic normalizes Arabic-script text into a lossy comparison key:
// presentation and selected language forms are handled before Unicode NFD,
// then tashkil and other combining marks, tatweel, and selected orthographic
// variants are folded. An opt-in rasm transform removes i'jam distinctions
// from Arabic consonants. Non-Arabic characters keep their spelling.
//
// Normalization is intentionally lossy: it is meant for tolerant search, not
// for round-tripping or for distinguishing text that a strict reading would
// keep apart.
package arabic

//go:generate go run ./gen -in gen/UnicodeData.txt -out tables.go
