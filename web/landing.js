/** agrep landing page interactive behaviors **/
"use strict";

document.addEventListener("DOMContentLoaded", () => {
  // 1. Copy install command
  const copyInstallBtn = document.getElementById("copy-install-btn");
  const installCmdEl = document.getElementById("install-command");

  if (copyInstallBtn && installCmdEl) {
    copyInstallBtn.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(installCmdEl.textContent.trim());
        const copyText = copyInstallBtn.querySelector(".copy-text");
        if (copyText) {
          const original = copyText.textContent;
          copyText.textContent = "Copied!";
          setTimeout(() => {
            copyText.textContent = original;
          }, 2000);
        }
      } catch (e) {
        console.error("Clipboard error:", e);
      }
    });
  }

  // 2. Alt install buttons
  document.querySelectorAll(".alt-install-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const cmd = btn.getAttribute("data-cmd");
      if (!cmd) return;
      if (installCmdEl) {
        installCmdEl.textContent = cmd;
      }
      try {
        await navigator.clipboard.writeText(cmd);
        const originalText = btn.textContent;
        btn.textContent = "Copied to clipboard!";
        setTimeout(() => {
          btn.textContent = originalText;
        }, 2000);
      } catch (e) {
        console.error("Clipboard error:", e);
      }
    });
  });

  // 3. Interactive Comparison Demo
  const scenarios = [
    {
      name: "Tashkil (Vowels)",
      grepCmd: "$ grep 'مدرسه' corpus.txt",
      grepResult: "(No match found — diacritics block exact byte search)",
      agrepCmd: "$ agrep 'مدرسه' corpus.txt",
      agrepResult: 'هذه <mark>مَدْرَسَةٌ</mark> عريقة في المدينة.',
      explanation: '<strong>How it works:</strong> The query <code>مدرسه</code> automatically folds ta-marbuta (<code>ة</code> → <code>ه</code>) and strips vocalization diacritics (<em>fathah, dammah, tanwin</em>) during normalized key comparison, while preserving original source text and exact UTF-8 byte spans.',
    },
    {
      name: "PDF Ligatures",
      grepCmd: "$ grep 'كتاب' corpus.txt",
      grepResult: "(No match found — PDF text uses U+FEFB presentation forms)",
      agrepCmd: "$ agrep 'كتاب' corpus.txt",
      agrepResult: 'ﻻ يوجد <mark>ﻛِﺘـﺎﺏ</mark> في الغرفة.',
      explanation: '<strong>How it works:</strong> Text extracted from PDFs often contains Arabic presentation forms (e.g. <code>ﻻ</code> and contextual glyph <code>ﻛِﺘـﺎﺏ</code>). agrep decomposes 731 Unicode presentation-form codepoints and strips tatweel (<em>kashida</em>), mapping matches back to the original source line.',
    },
    {
      name: "Cross-Language (Persian)",
      grepCmd: "$ grep 'كتاب' corpus.txt",
      grepResult: "(No match found — Persian uses Keheh ک U+06A9, not Arabic ك U+0643)",
      agrepCmd: "$ agrep --lang=ar,fa 'كتاب' corpus.txt",
      agrepResult: 'این <mark>کتاب</mark> تازه و بسیار آموزنده است.',
      explanation: '<strong>How it works:</strong> In multi-lingual archives, Persian and Urdu texts use distinct letter codepoints (<code>ک</code> and <code>ی</code>). With <code>--lang=ar,fa</code>, agrep enables shared-letter equivalence without conflating language-specific letters like ZWNJ or precomposed ezafe.',
    },
    {
      name: "Dotless Rasm (Manuscripts)",
      grepCmd: "$ grep 'مستشرق' corpus.txt",
      grepResult: "(No match found — manuscript is written in early dotless rasm)",
      agrepCmd: "$ agrep --rasm 'مستشرق' manuscript.txt",
      agrepResult: 'هذا <mark>مسٮسرٯ</mark> قديم من علماء القرن الماضي.',
      explanation: '<strong>How it works:</strong> Early Arabic manuscripts omit dots (<em>i\'jam</em>). agrep\'s <code>--rasm</code> mode folds the 9 consonant classes (e.g. <code>ب ت ث ن ي</code> → <code>ٮ</code>, <code>ف ق</code> → <code>ٯ</code>) into unified skeletal keys for high-recall historical search.',
    },
    {
      name: "Fuzzy Levenshtein",
      grepCmd: "$ grep 'خوارزمي' corpus.txt",
      grepResult: "(No match found — original text contains OCR error or spelling variation)",
      agrepCmd: "$ agrep --fuzzy=1 'خوارزمي' scans.txt",
      agrepResult: 'هو محمد بن موسى <mark>الخوارزمى</mark> عالم الرياضيات الشهير.',
      explanation: '<strong>How it works:</strong> Using bit-parallel Myers algorithms, <code>--fuzzy=1</code> permits up to 1 codepoint insertion, deletion, or substitution over normalized text. Ideal for OCR scanning noise, transcription errors, or regional spelling variants.',
    },
  ];

  const demoTabs = document.querySelectorAll(".demo-tab");
  const grepCmdEl = document.querySelector("#grep-box .demo-cli-cmd");
  const grepResultEl = document.querySelector("#grep-box .demo-result-text");
  const agrepCmdEl = document.querySelector("#agrep-box .demo-cli-cmd");
  const agrepMatchEl = document.getElementById("agrep-match-text");
  const demoExplanationEl = document.getElementById("demo-explanation");

  demoTabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      demoTabs.forEach((t) => t.classList.remove("active"));
      tab.classList.add("active");

      const idx = parseInt(tab.getAttribute("data-scenario"), 10);
      const sc = scenarios[idx];
      if (!sc) return;

      if (grepCmdEl) grepCmdEl.textContent = sc.grepCmd;
      if (grepResultEl) grepResultEl.textContent = sc.grepResult;
      if (agrepCmdEl) agrepCmdEl.textContent = sc.agrepCmd;
      if (agrepMatchEl) agrepMatchEl.innerHTML = sc.agrepResult;
      if (demoExplanationEl) demoExplanationEl.innerHTML = sc.explanation;
    });
  });
});
