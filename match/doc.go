// Package match finds occurrences of a search pattern inside text that has
// already been normalized by package arabic. Literal, regular-expression, and
// bit-parallel fuzzy matchers share normalized-key span and Profile contracts.
// A Matcher is built once from a query and reused across many lines, so query
// normalization and validation happen a single time rather than per line.
package match
