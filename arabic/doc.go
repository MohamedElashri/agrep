// Package arabic normalizes Arabic-script text into a lossy comparison key:
// Unicode NFD is applied first so canonically equivalent spellings become
// identical, then tashkil and other combining marks, tatweel, and a set of
// common orthographic variants (Alef, Hamza-seat, Ta-Marbuta, Alef-Maksura)
// are folded away. Non-Arabic characters keep their spelling.
//
// Normalization is intentionally lossy: it is meant for tolerant search, not
// for round-tripping or for distinguishing text that a strict reading would
// keep apart.
package arabic
