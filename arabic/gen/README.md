# Vendored Unicode Character Database

`UnicodeData.txt` in this directory is vendored from the Unicode Character
Database (UCD) so that `go generate` is reproducible without a network
dependency, and so the input to `tables.go`'s generation can be diffed
directly against upstream.

- **Source:** https://www.unicode.org/Public/18.0.0/ucd/UnicodeData.txt
- **Unicode version:** 18.0.0
- **Fetched:** 2026-09-21
- **License:** Unicode License v3 (`LICENSE` in this directory); permissive,
  redistribution allowed provided the copyright and permission notice is
  kept with the file, which it is.

## Updating

To pick up a newer Unicode version:

1. Download the new `UnicodeData.txt` from
   `https://www.unicode.org/Public/<version>/ucd/UnicodeData.txt` over this
   file.
2. Update the version/date above.
3. Run `go generate ./arabic/...` from the module root and review the diff
   to `../tables.go`. A new Unicode version can add, but should never
   remove, presentation-form entries in the ranges this generator reads
   (U+FB50–U+FDFF, U+FE70–U+FEFF), so a shrinking table is worth
   double-checking before committing.
4. Re-run the full test suite, including the fuzz targets. A changed table
   doesn't change the *normalization algorithm*, but it's cheap insurance.
