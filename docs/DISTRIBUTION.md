---
title: "Distribution & Releases"
description: "Release checklist, GoReleaser automation, multi-architecture binaries, and packaging recipes."
category: "Contributing & Project"
order: 2
---

# Distribution and release checklist

The repository has a tag-triggered GoReleaser workflow and a CI snapshot build.
Archives cover Linux, macOS, Windows, FreeBSD, OpenBSD, and NetBSD on amd64
and arm64, plus DragonFly BSD on amd64. Windows archives are zip files; the
rest are tar.gz. Windows builds are cross-compiled and untested: CI runs on
Linux only, and `scripts/install.sh` does not support Windows. Say so in the
release notes until a Windows CI job exists. Each archive contains the
binary, README, and license; `checksums.txt` contains SHA-256 hashes. Use a
`v0.x.y` tag while the public Go API is still under review.

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
5. The install script, README, and site install the latest release, so no
   version needs bumping. Check that the install script picks up the new tag:

   ```sh
   curl -fsSL https://raw.githubusercontent.com/MohamedElashri/agrep/main/scripts/install.sh | BINDIR="$(mktemp -d)" bash
   ```

The current GoReleaser configuration ships the dependency-light default CLI.
HTML and EPUB extraction require a separate source build with `-tags formats`;
those features are not advertised as part of the release archives. PDF and
DOCX extraction are still deferred.

The WebAssembly playground has its own build and Pages workflow. See
[PLAYGROUND.md](PLAYGROUND.md) for preview and deployment setup.
