/** agrep global site interactions: theme switching, smart clipboard, and syntax highlighting **/
"use strict";

(function () {
  // 1. Theme State & Switching
  const THEME_KEY = "agrep-theme";
  const root = document.documentElement;

  function getPreferredTheme() {
    const saved = localStorage.getItem(THEME_KEY);
    if (saved === "light" || saved === "dark") return saved;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function applyTheme(theme) {
    root.setAttribute("data-theme", theme);
    localStorage.setItem(THEME_KEY, theme);
  }

  // Initialize theme early to avoid FOUC
  applyTheme(getPreferredTheme());

  // 2. Syntax Highlighting Utility
  function escapeHtml(str) {
    return str
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#x27;");
  }

  const KNOWN_SHELL_CMDS = new Set([
    "agrep", "grep", "ripgrep", "rg", "curl", "bash", "sh", "go",
    "git", "gh", "node", "python3", "tar", "sudo", "mv", "cp",
    "mkdir", "printf", "cat", "echo", "chmod", "export", "set"
  ]);

  function highlightShell(code) {
    const lines = code.split("\n");
    const out = [];

    const tokenRe = new RegExp(
      "(?P<prompt>^\\s*[\\$#]\\s+)" +
      "|(?P<comment>#.*$)" +
      "|(?P<str>\"(?:[^\"\\\\]|\\\\.)*\"|'[^']*')" +
      "|(?P<flag>--?[a-zA-Z0-9_\\-]+(?:=[^\\s'\"]+)?)" +
      "|(?P<op>\\|{1,2}|&&|>>?|<<?|;)" +
      "|(?P<var>\\$[a-zA-Z_0-9]+|[A-Z_]{2,}=(?=\\S))" +
      "|(?P<word>[^\\s\"'|><;]+)" +
      "|(?P<space>\\s+)",
      "g"
    );

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed) {
        out.push("");
        continue;
      }
      if (trimmed.startsWith("#") && !trimmed.startsWith("#!")) {
        out.push(`<span class="tok-comment">${escapeHtml(line)}</span>`);
        continue;
      }

      let isFirstWord = true;
      let lineHtml = "";
      let lastIdx = 0;

      // Match tokens using regex
      const re = /(^\s*[\$#]\s+)|(#.*$)|("(?:[^"\\]|\\.)*"|'[^']*')|(--?[a-zA-Z0-9_\-]+(?:=[^\s'"]+)?)|(\|{1,2}|&&|>>?|<<?|;)|(\$[a-zA-Z_0-9]+|[A-Z_]{2,}=(?=\S))|([^\s"'|><;]+)|(\s+)/g;
      let m;

      while ((m = re.exec(line)) !== null) {
        const val = m[0];
        const esc = escapeHtml(val);

        if (m[1]) {
          // Prompt ($ or #)
          lineHtml += `<span class="tok-prompt">${esc}</span>`;
          isFirstWord = true;
        } else if (m[2]) {
          // Comment
          lineHtml += `<span class="tok-comment">${esc}</span>`;
        } else if (m[3]) {
          // String
          lineHtml += `<span class="tok-str">${esc}</span>`;
          isFirstWord = false;
        } else if (m[4]) {
          // Flag / option
          lineHtml += `<span class="tok-flag">${esc}</span>`;
          isFirstWord = false;
        } else if (m[5]) {
          // Pipe / operator
          lineHtml += `<span class="tok-punct">${esc}</span>`;
          isFirstWord = true;
        } else if (m[6]) {
          // Variable / env assignment
          lineHtml += `<span class="tok-var">${esc}</span>`;
        } else if (m[7]) {
          // Word / command / argument
          if (isFirstWord || KNOWN_SHELL_CMDS.has(val)) {
            lineHtml += `<span class="tok-cmd">${esc}</span>`;
          } else {
            lineHtml += esc;
          }
          isFirstWord = false;
        } else {
          // Whitespace
          lineHtml += esc;
        }
      }
      out.push(lineHtml);
    }
    return out.join("\n");
  }

  // Export to global for dynamic components (landing.js demo tabs)
  window.agrepHighlight = {
    escapeHtml,
    highlightShell,
  };

  // 3. Document interactions
  document.addEventListener("DOMContentLoaded", () => {
    // Theme toggle button
    const toggleBtn = document.getElementById("theme-toggle");
    if (toggleBtn) {
      toggleBtn.addEventListener("click", () => {
        const current = root.getAttribute("data-theme") || "light";
        const next = current === "dark" ? "light" : "dark";
        applyTheme(next);
      });
    }

    // Universal smart clipboard copy handler
    document.querySelectorAll(".copy-btn, .install-copy-btn, [data-copy-target]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        let textToCopy = "";
        const targetId = btn.getAttribute("data-copy-target");
        if (targetId) {
          const el = document.getElementById(targetId);
          if (el) textToCopy = el.textContent || "";
        } else {
          const codeBlock = btn.closest(".code-block, .code-showcase, .install-bar");
          if (codeBlock) {
            const codeEl = codeBlock.querySelector("code") || codeBlock.querySelector("pre");
            if (codeEl) textToCopy = codeEl.textContent || "";
          }
        }

        if (!textToCopy) return;

        // Clean command line: strip leading $ or # prompt so user can paste directly into shell
        const lines = textToCopy.split("\n");
        if (lines.some((l) => /^\s*[\$#]\s+/.test(l))) {
          textToCopy = lines.map((l) => l.replace(/^\s*[\$#]\s+/, "")).join("\n");
        }
        textToCopy = textToCopy.trim();

        try {
          await navigator.clipboard.writeText(textToCopy);
          const originalText = btn.innerHTML;
          const copyTextSpan = btn.querySelector(".copy-text");

          if (copyTextSpan) {
            const origSpan = copyTextSpan.textContent;
            copyTextSpan.textContent = "Copied!";
            btn.classList.add("copied");
            setTimeout(() => {
              copyTextSpan.textContent = origSpan;
              btn.classList.remove("copied");
            }, 2000);
          } else {
            btn.textContent = "Copied!";
            btn.classList.add("copied");
            setTimeout(() => {
              btn.innerHTML = originalText;
              btn.classList.remove("copied");
            }, 2000);
          }
        } catch (err) {
          console.error("Clipboard copy failed:", err);
        }
      });
    });
  });
})();
