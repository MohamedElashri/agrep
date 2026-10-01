/** agrep documentation interactive features: search, TOC scrollspy, sidebar filter **/
"use strict";

document.addEventListener("DOMContentLoaded", () => {
  // 1. Mobile Sidebar Drawer
  const sidebarToggleBtn = document.getElementById("sidebar-toggle");
  const docsSidebar = document.getElementById("docs-sidebar");

  if (sidebarToggleBtn && docsSidebar) {
    sidebarToggleBtn.addEventListener("click", () => {
      docsSidebar.classList.toggle("open");
    });

    document.addEventListener("click", (e) => {
      if (!docsSidebar.contains(e.target) && !sidebarToggleBtn.contains(e.target)) {
        docsSidebar.classList.remove("open");
      }
    });
  }

  // 2. Sidebar Quick Filter
  const filterInput = document.getElementById("doc-filter");
  const sidebarLinks = document.querySelectorAll(".sidebar-links li");

  if (filterInput) {
    filterInput.addEventListener("input", () => {
      const q = filterInput.value.toLowerCase().trim();
      sidebarLinks.forEach((item) => {
        const text = item.textContent.toLowerCase();
        const matches = text.includes(q);
        item.style.display = matches ? "" : "none";
      });

      // Hide empty groups
      document.querySelectorAll(".sidebar-group").forEach((group) => {
        const visibleItems = group.querySelectorAll(".sidebar-links li:not([style*='display: none'])");
        group.style.display = visibleItems.length > 0 ? "" : "none";
      });
    });
  }

  // 3. Table of Contents Scrollspy
  const tocLinks = document.querySelectorAll(".toc-list a");
  const headings = Array.from(document.querySelectorAll(".doc-body h2, .doc-body h3"));

  if (tocLinks.length > 0 && headings.length > 0) {
    function updateToc() {
      const scrollY = window.scrollY;
      let activeHeading = null;

      for (const h of headings) {
        if (h.offsetTop - 100 <= scrollY) {
          activeHeading = h;
        } else {
          break;
        }
      }

      if (activeHeading) {
        const slug = activeHeading.id;
        tocLinks.forEach((link) => {
          const href = link.getAttribute("href");
          if (href === `#${slug}`) {
            link.classList.add("active");
          } else {
            link.classList.remove("active");
          }
        });
      }
    }

    window.addEventListener("scroll", updateToc, { passive: true });
    updateToc();
  }

  // 4. Client-side Search Modal with search-index.json
  let searchIndex = null;
  const searchModal = document.getElementById("search-modal");
  const modalInput = document.getElementById("modal-search-input");
  const modalResults = document.getElementById("modal-search-results");

  async function loadSearchIndex() {
    if (searchIndex) return searchIndex;
    try {
      const res = await fetch("search-index.json");
      if (res.ok) {
        searchIndex = await res.json();
      }
    } catch (e) {
      console.warn("Could not fetch search index:", e);
    }
    return searchIndex || [];
  }

  function openSearch() {
    if (!searchModal) return;
    searchModal.classList.add("open");
    searchModal.setAttribute("aria-hidden", "false");
    loadSearchIndex();
    if (modalInput) {
      modalInput.value = "";
      modalInput.focus();
      renderSearchResults("");
    }
  }

  function closeSearch() {
    if (!searchModal) return;
    searchModal.classList.remove("open");
    searchModal.setAttribute("aria-hidden", "true");
  }

  function renderSearchResults(query) {
    if (!modalResults) return;
    const q = query.toLowerCase().trim();

    if (!q) {
      modalResults.innerHTML = '<div class="no-results">Type a keyword to search documentation...</div>';
      return;
    }

    if (!searchIndex) {
      modalResults.innerHTML = '<div class="no-results">Loading index...</div>';
      return;
    }

    const matches = [];
    for (const doc of searchIndex) {
      const titleMatch = doc.title.toLowerCase().includes(q);
      const descMatch = (doc.description || "").toLowerCase().includes(q);
      const matchedHeadings = (doc.headings || []).filter((h) => h.title.toLowerCase().includes(q));

      if (titleMatch || descMatch || matchedHeadings.length > 0) {
        matches.push({ doc, matchedHeadings });
      }
    }

    if (matches.length === 0) {
      modalResults.innerHTML = `<div class="no-results">No documentation matching "<strong>${escapeHtml(q)}</strong>"</div>`;
      return;
    }

    const htmlArr = [];
    for (const m of matches) {
      const doc = m.doc;
      htmlArr.push(`
        <a href="${doc.url}" class="search-result-item">
          <div class="search-result-title">
            <span>${escapeHtml(doc.title)}</span>
            <span class="search-result-cat">${escapeHtml(doc.category)}</span>
          </div>
          ${doc.description ? `<div class="search-result-desc">${escapeHtml(doc.description)}</div>` : ""}
        </a>
      `);

      for (const h of m.matchedHeadings.slice(0, 3)) {
        htmlArr.push(`
          <a href="${doc.url}#${h.slug}" class="search-result-item" style="padding-left: 1.8rem;">
            <div class="search-result-title" style="font-size: 0.88rem; font-weight: normal;">
              <span># ${escapeHtml(h.title)}</span>
              <span class="search-result-cat" style="opacity: 0.7;">Section</span>
            </div>
          </a>
        `);
      }
    }

    modalResults.innerHTML = htmlArr.join("");
  }

  function escapeHtml(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  // Keyboard shortcut listeners
  document.addEventListener("keydown", (e) => {
    if ((e.key === "/" || (e.key === "k" && (e.metaKey || e.ctrlKey))) && !["INPUT", "TEXTAREA"].includes(e.target.tagName)) {
      e.preventDefault();
      openSearch();
    } else if (e.key === "Escape" && searchModal && searchModal.classList.contains("open")) {
      closeSearch();
    }
  });

  if (searchModal) {
    const backdrop = searchModal.querySelector(".search-modal-backdrop");
    if (backdrop) backdrop.addEventListener("click", closeSearch);
  }

  if (modalInput) {
    modalInput.addEventListener("input", () => {
      renderSearchResults(modalInput.value);
    });
  }
});
