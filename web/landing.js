/** agrep landing page interactive behaviors **/
"use strict";

document.addEventListener("DOMContentLoaded", () => {
  // grep vs agrep example scenarios
  const scenarios = [
    {
      name: "Tashkil (Vowels)",
      grepCmd: "$ grep 'مدرسه' corpus.txt",
      grepResult: "(no match: the text has vowel marks)",
      agrepCmd: "$ agrep 'مدرسه' corpus.txt",
      agrepResult: 'هذه <mark>مَدْرَسَةٌ</mark> عريقة في المدينة.',
      agrepStatus: "1 match · bytes [7:27]",
      explanation: 'The default profile folds <code>ة</code> to <code>ه</code> and removes the vowel marks (<em>fathah</em>, <em>dammah</em>, <em>tanwin</em>) before comparing. The highlighted word is the original text, vowels included.',
    },
    {
      name: "PDF Ligatures",
      grepCmd: "$ grep 'كتاب' corpus.txt",
      grepResult: "(no match: the word is stored as presentation forms)",
      agrepCmd: "$ agrep 'كتاب' corpus.txt",
      agrepResult: 'ﻻ يوجد <mark>ﻛِﺘـﺎﺏ</mark> في الغرفة.',
      agrepStatus: "1 match · bytes [13:29]",
      explanation: 'Text copied from PDFs often stores contextual glyphs such as <code>ﻛ</code> (U+FEDB) and ligatures such as <code>ﻻ</code> instead of ordinary letters. agrep expands 731 presentation-form codepoints and removes tatweel (<em>kashida</em>) before comparing.',
    },
    {
      name: "Cross-Language (Persian)",
      grepCmd: "$ grep 'كتاب' corpus.txt",
      grepResult: "(no match: Persian uses ک U+06A9, not ك U+0643)",
      agrepCmd: "$ agrep --lang=ar,fa 'كتاب' corpus.txt",
      agrepResult: 'این <mark>کتاب</mark> تازه و بسیار آموزنده است.',
      agrepStatus: "1 match · bytes [7:15]",
      explanation: 'Persian writes <code>ک</code> and <code>ی</code> where Arabic writes <code>ك</code> and <code>ي</code>. With <code>--lang=ar,fa</code>, agrep treats each pair as the same letter. Persian rules such as keeping ZWNJ still apply.',
    },
    {
      name: "Dotless Rasm (Manuscripts)",
      grepCmd: "$ grep 'مستشرق' corpus.txt",
      grepResult: "(no match: the manuscript text has no dots)",
      agrepCmd: "$ agrep --rasm 'مستشرق' manuscript.txt",
      agrepResult: 'هذا <mark>مسٮسرٯ</mark> قديم من علماء القرن الماضي.',
      agrepStatus: "1 match · bytes [7:19]",
      explanation: 'Early manuscripts omit the dots (<em>i\'jam</em>) that tell letters apart. <code>--rasm</code> folds nine groups of letters to one key each, for example <code>ب ت ث ن ي</code> → <code>ٮ</code> and <code>ف ق</code> → <code>ٯ</code>. Expect extra matches.',
    },
    {
      name: "Fuzzy Levenshtein",
      grepCmd: "$ grep 'خوارزمي' corpus.txt",
      grepResult: "(no match: the text is missing ر)",
      agrepCmd: "$ agrep --fuzzy=1 'خوارزمي' scans.txt",
      agrepResult: 'هو محمد بن موسى ال<mark>خوازمي</mark> عالم الرياضيات الشهير.',
      agrepStatus: "1 match · bytes [32:44]",
      explanation: '<code>--fuzzy=1</code> allows one inserted, deleted, or substituted codepoint, counted after normalization. Use it for OCR output and typos.',
    },
  ];

  const demoTabs = Array.from(document.querySelectorAll(".demo-tab"));
  const grepCmdEl = document.getElementById("grep-cmd");
  const grepResultEl = document.getElementById("grep-result");
  const agrepCmdEl = document.getElementById("agrep-cmd");
  const agrepMatchEl = document.getElementById("agrep-match-text");
  const demoExplanationEl = document.getElementById("demo-explanation");
  const agrepStatusEl = document.getElementById("agrep-status");

  function applyCmdHighlight(el, cmdText) {
    if (!el) return;
    if (window.agrepHighlight && typeof window.agrepHighlight.highlightShell === "function") {
      el.innerHTML = window.agrepHighlight.highlightShell(cmdText);
    } else {
      el.textContent = cmdText;
    }
  }

  function activateTab(index) {
    if (index < 0 || index >= scenarios.length) return;
    const sc = scenarios[index];
    if (!sc) return;

    demoTabs.forEach((t, i) => {
      const isActive = i === index;
      t.classList.toggle("active", isActive);
      t.setAttribute("aria-selected", isActive ? "true" : "false");
      t.setAttribute("tabindex", isActive ? "0" : "-1");
    });

    applyCmdHighlight(grepCmdEl, sc.grepCmd);
    if (grepResultEl) grepResultEl.textContent = sc.grepResult;
    applyCmdHighlight(agrepCmdEl, sc.agrepCmd);
    if (agrepMatchEl) agrepMatchEl.innerHTML = sc.agrepResult;
    if (agrepStatusEl) agrepStatusEl.textContent = sc.agrepStatus;
    if (demoExplanationEl) demoExplanationEl.innerHTML = sc.explanation;
  }

  // Initial syntax highlight on page load for scenario 0
  activateTab(0);

  // Click & keyboard navigation for tabs
  demoTabs.forEach((tab, idx) => {
    tab.addEventListener("click", () => activateTab(idx));

    tab.addEventListener("keydown", (e) => {
      let targetIdx = null;
      if (e.key === "ArrowRight") {
        targetIdx = (idx + 1) % demoTabs.length;
      } else if (e.key === "ArrowLeft") {
        targetIdx = (idx - 1 + demoTabs.length) % demoTabs.length;
      } else if (e.key === "Home") {
        targetIdx = 0;
      } else if (e.key === "End") {
        targetIdx = demoTabs.length - 1;
      }

      if (targetIdx !== null) {
        e.preventDefault();
        demoTabs[targetIdx].focus();
        activateTab(targetIdx);
      }
    });
  });
});
