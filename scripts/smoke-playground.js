"use strict";

const fs = require("node:fs");
const path = require("node:path");

const dist = path.resolve(__dirname, "../web/dist");
require(path.join(dist, "wasm_exec.js"));

async function main() {
  const go = new Go();
  const bytes = fs.readFileSync(path.join(dist, "agrep.wasm"));
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  void go.run(instance);
  for (let attempt = 0; attempt < 100 && typeof globalThis.agrepNormalize !== "function"; attempt++) {
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  if (typeof globalThis.agrepNormalize !== "function") throw new Error("WASM function not registered");
  const result = JSON.parse(globalThis.agrepNormalize("ﻻ كِتـاب", "search", "ar"));
  if (result.key !== "لا كتاب" || !result.rules.includes("presentation forms")) {
    throw new Error(`unexpected WASM result: ${JSON.stringify(result)}`);
  }
  console.log("Playground WASM smoke check passed");
  process.exit(0); // Go's main blocks to keep the exported function alive.
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
