/** agrep global site interactions: theme switching & clipboard **/
"use strict";

(function () {
  // Theme state
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

  // Initialize theme
  applyTheme(getPreferredTheme());

  // Attach theme toggle button
  document.addEventListener("DOMContentLoaded", () => {
    const toggleBtn = document.getElementById("theme-toggle");
    if (toggleBtn) {
      toggleBtn.addEventListener("click", () => {
        const current = root.getAttribute("data-theme") || "light";
        const next = current === "dark" ? "light" : "dark";
        applyTheme(next);
      });
    }

    // Generic copy buttons
    document.querySelectorAll(".copy-btn").forEach((btn) => {
      btn.addEventListener("click", async () => {
        let textToCopy = "";
        const targetId = btn.getAttribute("data-copy-target");
        if (targetId) {
          const el = document.getElementById(targetId);
          if (el) textToCopy = el.textContent || "";
        } else {
          // If inside a code-block, copy the pre code content
          const codeBlock = btn.closest(".code-block");
          if (codeBlock) {
            const codeEl = codeBlock.querySelector("code");
            if (codeEl) textToCopy = codeEl.textContent || "";
          }
        }

        if (!textToCopy) return;

        try {
          await navigator.clipboard.writeText(textToCopy);
          const originalText = btn.textContent;
          btn.textContent = "Copied!";
          btn.classList.add("copied");
          setTimeout(() => {
            btn.textContent = originalText;
            btn.classList.remove("copied");
          }, 2000);
        } catch (err) {
          console.error("Clipboard copy failed:", err);
        }
      });
    });
  });
})();
