#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${repo_root}/web/dist}"
mkdir -p "$output_dir" "$output_dir/playground"

# 1. Build the WebAssembly binary from cmd/playground
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$output_dir/"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$output_dir/playground/"
GOOS=js GOARCH=wasm go build -trimpath -o "$output_dir/agrep.wasm" "$repo_root/cmd/playground"
cp "$output_dir/agrep.wasm" "$output_dir/playground/"

# 2. Build landing page, HTML documentation, and playground pages
python3 "$repo_root/scripts/build-site.py" --repo-root "$repo_root" --output-dir "$output_dir"

printf 'Successfully built agrep website, docs, and playground at %s\n' "$output_dir"
