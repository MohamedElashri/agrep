#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${repo_root}/web/dist}"
mkdir -p "$output_dir"
cp "$repo_root/web/index.html" "$repo_root/web/app.js" "$repo_root/web/style.css" "$repo_root/web/GO-LICENSE.txt" "$output_dir/"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$output_dir/"
GOOS=js GOARCH=wasm go build -trimpath -o "$output_dir/agrep.wasm" "$repo_root/cmd/playground"
printf 'Built playground at %s\n' "$output_dir"
