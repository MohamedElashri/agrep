"use strict";

const input = document.getElementById("input");
const profile = document.getElementById("profile");
const languages = document.getElementById("languages");
const key = document.getElementById("key");
const rules = document.getElementById("rules");
const status = document.getElementById("status");
const clear = document.getElementById("clear");
const copy = document.getElementById("copy");
let currentKey = "";

function refresh() {
  if (typeof window.agrepNormalize !== "function") return;
  const result = JSON.parse(window.agrepNormalize(input.value, profile.value, languages.value));
  if (result.error) {
    currentKey = "";
    key.textContent = "";
    key.classList.remove("changed");
    copy.disabled = true;
    rules.replaceChildren();
    status.textContent = result.error;
    return;
  }
  currentKey = result.key;
  copy.disabled = !currentKey;
  key.textContent = result.key || "∅ (empty key)";
  key.classList.toggle("changed", result.key !== input.value);
  rules.replaceChildren();
  for (const rule of result.rules || []) {
    const chip = document.createElement("span");
    chip.className = "rule";
    chip.textContent = rule;
    rules.appendChild(chip);
  }
  if (rules.childElementCount === 0) {
    const note = document.createElement("span");
    note.className = "no-rules";
    note.textContent = "No selected rule changes this text.";
    rules.appendChild(note);
  }
  status.textContent = "Ready. Changes update as you type.";
}

for (const element of [input, profile, languages]) {
  element.addEventListener(element === input ? "input" : "change", refresh);
}
for (const button of document.querySelectorAll("[data-example]")) {
  button.addEventListener("click", () => {
    input.value = button.dataset.example;
    profile.value = button.dataset.profile;
    languages.value = button.dataset.languages;
    refresh();
    input.focus();
  });
}
clear.addEventListener("click", () => {
  input.value = "";
  refresh();
  input.focus();
});
copy.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(currentKey);
    status.textContent = "Comparison key copied to clipboard.";
  } catch {
    status.textContent = "Could not copy the key. Select the result to copy it manually.";
  }
});

async function start() {
  try {
    const go = new Go();
    const response = await fetch("agrep.wasm");
    if (!response.ok) throw new Error(`WASM download failed (${response.status})`);
    const wasm = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
    void go.run(wasm.instance);
    for (let attempt = 0; attempt < 100 && typeof window.agrepNormalize !== "function"; attempt++) {
      await new Promise(resolve => setTimeout(resolve, 10));
    }
    if (typeof window.agrepNormalize !== "function") throw new Error("WASM initialization timed out");
    refresh();
  } catch (error) {
    status.textContent = `Could not start the playground: ${error.message}`;
    key.textContent = "Unavailable";
    copy.disabled = true;
  }
}

start();
