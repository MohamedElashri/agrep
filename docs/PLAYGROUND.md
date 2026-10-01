---
title: "Normalization Playground"
description: "Interactive WebAssembly browser tool for testing Arabic Unicode normalization rules and comparison keys."
category: "Tools & Ecosystem"
order: 1
---

# Normalization playground

The browser playground uses the same Go normalization code as the CLI. Enter
text, choose a profile and language, and inspect or copy the comparison key.
Use an example to load its text and settings, or Clear to start over. A rule
chip is highlighted when disabling that rule alone changes the key; it is not
a full transformation trace. Input stays in the browser and is limited to
32 KiB per example.

## Local preview

```sh
bash scripts/build-playground.sh
node scripts/smoke-playground.js
python3 -m http.server 8080 --directory web/dist
```

Open `http://localhost:8080/` to browse the landing page, `http://localhost:8080/docs/` for HTML documentation, or `http://localhost:8080/playground/` for the interactive playground. Serve over HTTP so the browser can fetch `agrep.wasm`. The build includes the Go toolchain's `wasm_exec.js` and its [license](../web/GO-LICENSE.txt); generated `web/dist` is not tracked.

The Pages workflow builds the same directory from `main`. Configure the repository's Pages source to **GitHub Actions** as described in the [GitHub Pages guide](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).
