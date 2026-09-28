#!/usr/bin/env python3
"""Generate Homebrew and AUR recipes from a GoReleaser checksum file."""

import argparse
import re
from pathlib import Path


REPOSITORY = "https://github.com/MohamedElashri/agrep"
PLATFORMS = {
    "darwin_amd64": "darwin_amd64",
    "darwin_arm64": "darwin_arm64",
    "linux_amd64": "linux_amd64",
    "linux_arm64": "linux_arm64",
}


def read_checksums(path):
    checksums = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9_.-]+)", line)
        if not match:
            raise ValueError(f"invalid checksum line: {line!r}")
        if match.group(2) in checksums:
            raise ValueError(f"duplicate checksum entry: {match.group(2)}")
        checksums[match.group(2)] = match.group(1)
    return checksums


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version", help="release version without the v prefix, e.g. 0.1.0")
    parser.add_argument("checksums", type=Path, help="GoReleaser checksums.txt")
    parser.add_argument("output", type=Path, help="directory for agrep.rb and PKGBUILD")
    args = parser.parse_args()
    if not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", args.version):
        parser.error("version must be a stable SemVer triple without a v prefix")
    checksums = read_checksums(args.checksums)
    archive = {platform: f"agrep_{args.version}_{suffix}.tar.gz" for platform, suffix in PLATFORMS.items()}
    missing = [name for name in archive.values() if name not in checksums]
    if missing:
        parser.error(f"missing checksums for: {', '.join(missing)}")

    def url(platform):
        return f"{REPOSITORY}/releases/download/v{args.version}/{archive[platform]}"

    def brew_branch(platform, indent):
        return f'{indent}url "{url(platform)}"\n{indent}sha256 "{checksums[archive[platform]]}"'

    formula = f'''class Agrep < Formula
  desc "Arabic-script text search with Unicode normalization"
  homepage "{REPOSITORY}"
  version "{args.version}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
{brew_branch("darwin_arm64", "      ")}
    else
{brew_branch("darwin_amd64", "      ")}
    end
  end

  on_linux do
    if Hardware::CPU.arm?
{brew_branch("linux_arm64", "      ")}
    else
{brew_branch("linux_amd64", "      ")}
    end
  end

  def install
    bin.install "agrep"
  end

  test do
    assert_match "agrep", shell_output("#{{bin}}/agrep --version")
  end
end
'''
    aur = f'''pkgname=agrep-bin
pkgver={args.version}
pkgrel=1
pkgdesc='Arabic-script text search with Unicode normalization'
arch=('x86_64' 'aarch64')
url='{REPOSITORY}'
license=('MIT')
source_x86_64=('{url("linux_amd64")}')
source_aarch64=('{url("linux_arm64")}')
sha256sums_x86_64=('{checksums[archive["linux_amd64"]]}')
sha256sums_aarch64=('{checksums[archive["linux_arm64"]]}')

package() {{
  install -Dm755 "$srcdir/agrep" "$pkgdir/usr/bin/agrep"
  install -Dm644 "$srcdir/LICENSE" "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}}
'''
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / "agrep.rb").write_text(formula, encoding="utf-8")
    (args.output / "PKGBUILD").write_text(aur, encoding="utf-8")
    print(f"Wrote {args.output / 'agrep.rb'} and {args.output / 'PKGBUILD'}")


if __name__ == "__main__":
    main()
