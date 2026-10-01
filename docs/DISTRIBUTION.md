---
title: "Distribution & Releases"
description: "Release checklist, GoReleaser automation, multi-architecture binaries, and packaging recipes."
category: "Contributing & Project"
order: 2
---

# Distribution and release checklist

The repository has a tag-triggered GoReleaser workflow and a CI snapshot build.
Archives cover Linux, macOS, FreeBSD, OpenBSD, and NetBSD on amd64 and arm64,
plus DragonFly BSD on amd64. Each archive contains the binary, README, and
license; `checksums.txt` contains SHA-256 hashes. Use a `v0.x.y` tag while
the public Go API is still under review.

For each release:

1. Merge a reviewed commit to the repository's default branch (`main`). Run
   the CI tests and GoReleaser snapshot build there.
2. Create and push an annotated SemVer tag on that commit. The release workflow
   verifies that it belongs to the default branch, then publishes the archives
   and `checksums.txt`. For example:

   ```sh
   git tag -a v0.1.0 -m "agrep v0.1.0"
   git push origin v0.1.0
   ```

   Prerelease tags such as `v0.1.0-rc.1` create GitHub prereleases. GoReleaser
   derives notes from commits since the previous tag and embeds the version
   without `v`; verify it with `agrep --version`.

3. Download the release's `checksums.txt`. Generate package recipes from its
   real SHA-256 values:

   ```sh
   python3 scripts/package-release.py 0.1.0 checksums.txt dist/packages
   ```

4. Check the archive names and install both recipes on supported machines.
   Publish `agrep.rb` to a Homebrew tap and `PKGBUILD` to the AUR after this
   verification. The generator writes files locally; it does not publish or
   create accounts.
5. After a tag is live, document a pinned install command such as
   `go install github.com/MohamedElashri/agrep/cmd/agrep@v0.1.0`.

The current GoReleaser configuration ships the dependency-light default CLI.
HTML and EPUB extraction require a separate source build with `-tags formats`;
those features are not advertised as part of the release archives. PDF and
DOCX extraction are still deferred.

The WebAssembly playground has its own build and Pages workflow. See
[PLAYGROUND.md](PLAYGROUND.md) for preview and deployment setup.
