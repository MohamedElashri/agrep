// Package scan reads arbitrary-length logical lines from an io.Reader and
// reports the ones whose normalized text a match.Matcher finds, without
// bufio.Scanner's token-size ceiling.
package scan
