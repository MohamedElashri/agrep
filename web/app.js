"use strict";

const input = document.getElementById("input");
const profile = document.getElementById("profile");
const languages = document.getElementById("languages");
const key = document.getElementById("key");
const rules = document.getElementById("rules");
const status = document.getElementById("status");

function refresh() {
  if (typeof window.agrepNormalize !== "function") return;
  const result = JSON.parse(window.agrepNormalize(input.value, profile.value, languages.value));
  if (result.error) {
    key.textContent = "";
    rules.replaceChildren();
    status.textContent = result.error;
    return;
  }
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
    if (button.textContent.includes("rasm")) profile.value = "loose";
    if (button.textContent.includes("Persian")) languages.value = "ar,fa";
    refresh();
    input.focus();
  });
}

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
  }
}

start();
