# Normalization playground

`web/` is a static interface over the same `arabic.Profile.Normalize` code
used by the CLI. It lets visitors enter text, choose a built-in profile and
language, and see the resulting comparison key. The highlighted rule chips
mean that disabling that rule alone changes the final key. When rules overlap,
an effective rule can be absent from the list because another enabled rule
produces the same key; this is a diagnostic view, not a full transformation
trace.

Build and preview locally:

```sh
bash scripts/build-playground.sh
node scripts/smoke-playground.js
python3 -m http.server 8080 --directory web/dist
```

Then open `http://localhost:8080/`. A local HTTP server is required for the
browser to fetch `agrep.wasm`. The build copies the `wasm_exec.js` matching the
installed Go toolchain into the output directory. That runtime is licensed by
the Go Authors under the [Go license](https://go.dev/LICENSE), reproduced in
`web/GO-LICENSE.txt` and included in the published site. The generated
`web/dist` is ignored by Git and rebuilt in CI and in the Pages workflow.

The site limits an example to 32 KiB. Input stays in the browser; the page
does not send it to a server. The Pages workflow deploys `web/dist` after
relevant changes land on `main`. The repository must have GitHub Pages
configured to deploy from GitHub Actions, as described in the
[GitHub Pages workflow guide](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).
Until the workflow runs successfully on `main`, the local build is the
available preview.
