#!/usr/bin/env python3
"""Static site generator for agrep landing page, HTML docs, and playground."""

import argparse
import html
import json
import os
import re
import shutil
import sys
from pathlib import Path

# Mapping of markdown filenames to HTML filenames
DOC_FILE_MAP = {
    "index.md": "index.html",
    "cli.md": "cli.html",
    "normalization.md": "normalization.html",
    "languages.md": "languages.html",
    "matching.md": "matching.html",
    "input.md": "input.html",
    "terminals.md": "terminals.html",
    "benchmarks.md": "benchmarks.html",
    "playground.md": "playground.html",
    "development.md": "development.html",
    "distribution.md": "distribution.html",
}

CATEGORY_ORDER = [
    "Getting Started",
    "Core Concepts",
    "Display & Output",
    "Performance",
    "Tools & Ecosystem",
    "Contributing & Project",
]

SITE_URL = "https://mohamedelashri.github.io/agrep"
DEFAULT_KEYWORDS = (
    "agrep, arabic search, arabic grep, unicode normalization, ripgrep, grep, "
    "tashkil, rasm, dotless rasm, persian search, urdu search, pashto, kurdish, "
    "uyghur, buckwalter, fuzzy search, cli, go, open source"
)


def generate_sitemap(flat_list: list[dict]) -> str:
    """Generate XML sitemap conforming to sitemaps.org protocol."""
    entries = [
        (f"{SITE_URL}/", "1.0", "weekly"),
        (f"{SITE_URL}/docs/", "0.9", "weekly"),
        (f"{SITE_URL}/playground/", "0.9", "weekly"),
    ]
    for doc in flat_list:
        if doc["filename"] != "index.html":
            entries.append((f"{SITE_URL}/docs/{doc['filename']}", "0.8", "weekly"))

    items = []
    for loc, priority, freq in entries:
        items.append(
            f"  <url>\n"
            f"    <loc>{loc}</loc>\n"
            f"    <changefreq>{freq}</changefreq>\n"
            f"    <priority>{priority}</priority>\n"
            f"  </url>"
        )

    return (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n'
        + "\n".join(items)
        + "\n</urlset>\n"
    )


def generate_robots_txt() -> str:
    """Generate robots.txt allowing indexing and pointing to sitemap."""
    return f"""User-agent: *
Allow: /

Sitemap: {SITE_URL}/sitemap.xml
"""


def slugify(text: str) -> str:
    """Generate a URL-friendly anchor slug from heading text."""
    # Strip HTML tags if any
    clean = re.sub(r"<[^>]+>", "", text)
    # Convert to lowercase
    clean = clean.lower().strip()
    # Replace non-alphanumeric (except dashes and spaces) with empty
    clean = re.sub(r"[^\w\s-]", "", clean, flags=re.UNICODE)
    # Replace spaces and underscores with dashes
    clean = re.sub(r"[\s_]+", "-", clean)
    clean = clean.strip("-")
    return clean or "section"


def parse_frontmatter(content: str):
    """Extract frontmatter and remaining markdown text."""
    if not content.startswith("---"):
        return {}, content

    parts = content.split("---", 2)
    if len(parts) < 3:
        return {}, content

    meta_str = parts[1].strip()
    body = parts[2].lstrip("\n")
    metadata = {}
    for line in meta_str.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" in line:
            key, val = line.split(":", 1)
            key = key.strip()
            val = val.strip().strip("\"'")
            if val.isdigit():
                val = int(val)
            metadata[key] = val

    return metadata, body


class MarkdownParser:
    """Zero-dependency Markdown to HTML parser tailored for technical docs."""

    def __init__(self, current_doc_name: str = ""):
        self.current_doc_name = current_doc_name.lower()
        self.ref_links = {}
        self.toc = []

    def remap_link(self, href: str) -> str:
        """Remap relative markdown file links to their compiled HTML counterparts."""
        # Handle reference links anchor
        anchor = ""
        url = href
        if "#" in url:
            url, anchor = url.split("#", 1)
            anchor = "#" + anchor

        url_lower = url.lower()
        # Handle docs/XYZ.md from outside or inside
        base_name = os.path.basename(url_lower)
        if base_name in DOC_FILE_MAP:
            target = DOC_FILE_MAP[base_name]
            return target + anchor

        if url in ("../README.md", "README.md", "../LICENSE", "LICENSE"):
            return "https://github.com/MohamedElashri/agrep" + anchor

        if (url.startswith("../arabic/") or url.startswith("../testdata/") or
                url.startswith("../scripts/") or url.startswith("../.agents/") or
                url.startswith("../.claude/")):
            clean_path = url.lstrip("./")
            return f"https://github.com/MohamedElashri/agrep/blob/main/{clean_path}{anchor}"

        if url == "../web/GO-LICENSE.txt":
            return f"../playground/GO-LICENSE.txt{anchor}"

        return href

    def parse_inline(self, text: str) -> str:
        """Parse inline markdown formatting (bold, italic, code, links)."""
        # 1. Protect inline code spans first
        code_spans = []

        def save_code(m):
            code_spans.append(m.group(1))
            return f"\x00INLINECODE{len(code_spans) - 1}\x00"

        text = re.sub(r"`([^`]+)`", save_code, text)

        # 2. Parse links and protect them from bold/italic parsing
        link_spans = []

        def save_link(html_str: str) -> str:
            link_spans.append(html_str)
            return f"\x00INLINELINK{len(link_spans) - 1}\x00"

        # Autolinks: <http://...>
        def replace_autolink(m):
            u = m.group(1)
            return save_link(f'<a href="{u}" target="_blank" rel="noopener noreferrer">{u}</a>')

        text = re.sub(r"<(https?://[^>]+)>", replace_autolink, text)

        # Inline links: [text](href)
        def replace_link(m):
            t = m.group(1)
            href = m.group(2).strip()
            final_href = self.remap_link(href)
            external = ' target="_blank" rel="noopener noreferrer"' if final_href.startswith("http") else ""
            return save_link(f'<a href="{final_href}"{external}>{self.parse_inline(t)}</a>')

        text = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", replace_link, text)

        # Reference links: [text][ref]
        def replace_ref_link(m):
            t = m.group(1)
            ref = (m.group(2) or t).strip().lower()
            if ref in self.ref_links:
                href = self.remap_link(self.ref_links[ref])
                external = ' target="_blank" rel="noopener noreferrer"' if href.startswith("http") else ""
                return save_link(f'<a href="{href}"{external}>{self.parse_inline(t)}</a>')
            return m.group(0)

        text = re.sub(r"\[([^\]]+)\]\[([^\]]*)\]", replace_ref_link, text)

        # 3. Bold & Italic
        text = re.sub(r"\*\*\*([^*]+)\*\*\*", r"<strong><em>\1</em></strong>", text)
        text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
        text = re.sub(r"__([^_]+)__", r"<strong>\1</strong>", text)
        text = re.sub(r"\*([^*]+)\*", r"<em>\1</em>", text)
        text = re.sub(r"_([^_]+)_", r"<em>\1</em>", text)

        # 4. Strikethrough
        text = re.sub(r"~~([^~]+)~~", r"<del>\1</del>", text)

        # 5. Restore link spans
        for idx, link_html in enumerate(link_spans):
            text = text.replace(f"\x00INLINELINK{idx}\x00", link_html)

        # 6. Restore code spans
        for idx, code in enumerate(code_spans):
            escaped = html.escape(code)
            text = text.replace(f"\x00INLINECODE{idx}\x00", f"<code>{escaped}</code>")

        return text

    def parse(self, markdown_text: str) -> tuple[str, list[dict]]:
        """Parse complete markdown document into HTML and TOC list."""
        self.toc = []
        self.ref_links = {}

        # First pass: collect reference links [ref]: url
        remaining_lines = []
        for line in markdown_text.splitlines():
            ref_match = re.match(r"^\s*\[([^\]]+)\]:\s*(\S+)(?:\s+.*)?$", line)
            if ref_match:
                self.ref_links[ref_match.group(1).strip().lower()] = ref_match.group(2).strip()
            else:
                remaining_lines.append(line)

        html_blocks = []
        i = 0
        n = len(remaining_lines)

        while i < n:
            line = remaining_lines[i]

            # Blank lines
            if not line.strip():
                i += 1
                continue

            # Fenced code block
            if line.strip().startswith("```"):
                fence = line.strip()[:3]
                info = line.strip()[3:].strip()
                lang = info.split()[0] if info else "text"
                code_lines = []
                i += 1
                while i < n and not remaining_lines[i].strip().startswith(fence):
                    code_lines.append(remaining_lines[i])
                    i += 1
                if i < n:
                    i += 1  # Skip closing fence
                code_raw = "\n".join(code_lines)
                escaped = html.escape(code_raw)
                html_blocks.append(
                    f'<div class="code-block" data-lang="{html.escape(lang)}">'
                    f'<div class="code-bar">'
                    f'<span class="code-lang">{html.escape(lang)}</span>'
                    f'<button class="copy-btn" type="button" aria-label="Copy code">Copy</button>'
                    f'</div>'
                    f'<pre><code class="language-{html.escape(lang)}">{escaped}</code></pre>'
                    f'</div>'
                )
                continue

            # Blockquote & Alerts
            if line.startswith(">"):
                quote_lines = []
                while i < n and (remaining_lines[i].startswith(">") or (quote_lines and remaining_lines[i].strip())):
                    raw_line = remaining_lines[i]
                    if raw_line.startswith(">"):
                        quote_lines.append(raw_line[1:].lstrip())
                    else:
                        quote_lines.append(raw_line)
                    i += 1

                # Check for GitHub-style callouts: [!NOTE], [!TIP], [!IMPORTANT], [!WARNING], [!CAUTION]
                first_line = quote_lines[0].strip() if quote_lines else ""
                alert_match = re.match(r"^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]$", first_line, re.IGNORECASE)
                if alert_match:
                    alert_type = alert_match.group(1).lower()
                    alert_content = "\n".join(quote_lines[1:])
                    inner_html, _ = MarkdownParser(self.current_doc_name).parse(alert_content)
                    title_map = {
                        "note": "Note",
                        "tip": "Tip",
                        "important": "Important",
                        "warning": "Warning",
                        "caution": "Caution",
                    }
                    html_blocks.append(
                        f'<div class="callout callout-{alert_type}">'
                        f'<div class="callout-title"><span class="callout-badge">{title_map.get(alert_type, "Note")}</span></div>'
                        f'<div class="callout-body">{inner_html}</div>'
                        f'</div>'
                    )
                else:
                    inner_html, _ = MarkdownParser(self.current_doc_name).parse("\n".join(quote_lines))
                    html_blocks.append(f'<blockquote>{inner_html}</blockquote>')
                continue

            # Headings: # H1, ## H2, etc.
            heading_match = re.match(r"^(#{1,6})\s+(.+)$", line)
            if heading_match:
                level = len(heading_match.group(1))
                heading_raw = heading_match.group(2).strip()
                # Parse inline formatting inside heading
                heading_html = self.parse_inline(heading_raw)
                heading_slug = slugify(heading_raw)

                if level in (2, 3):
                    self.toc.append({
                        "level": level,
                        "title": re.sub(r"<[^>]+>", "", heading_html),
                        "slug": heading_slug,
                    })

                anchor_link = f'<a href="#{heading_slug}" class="heading-anchor" aria-label="Link to {html.escape(heading_slug)}">#</a>'
                html_blocks.append(f'<h{level} id="{heading_slug}">{heading_html}{anchor_link}</h{level}>')
                i += 1
                continue

            # Horizontal Rule
            if re.match(r"^(-{3,}|\*{3,}|_{3,})$", line.strip()):
                html_blocks.append("<hr>")
                i += 1
                continue

            # Table: check for pipe table
            if line.strip().startswith("|") and line.strip().endswith("|") and i + 1 < n and "| ---" in remaining_lines[i + 1]:
                header_line = remaining_lines[i].strip()
                delimiter_line = remaining_lines[i + 1].strip()
                headers = [c.strip() for c in header_line.strip("|").split("|")]
                delimiters = [c.strip() for c in delimiter_line.strip("|").split("|")]

                alignments = []
                for d in delimiters:
                    if d.startswith(":") and d.endswith(":"):
                        alignments.append("center")
                    elif d.endswith(":"):
                        alignments.append("right")
                    else:
                        alignments.append("left")

                table_rows = []
                i += 2
                while i < n and remaining_lines[i].strip().startswith("|") and remaining_lines[i].strip().endswith("|"):
                    row_cells = [c.strip() for c in remaining_lines[i].strip("|").split("|")]
                    table_rows.append(row_cells)
                    i += 1

                th_html = "".join(
                    f'<th class="text-{alignments[idx] if idx < len(alignments) else "left"}">{self.parse_inline(h)}</th>'
                    for idx, h in enumerate(headers)
                )
                tbody_html = []
                for row in table_rows:
                    tr = []
                    for idx, cell in enumerate(row):
                        align = alignments[idx] if idx < len(alignments) else "left"
                        tr.append(f'<td class="text-{align}">{self.parse_inline(cell)}</td>')
                    tbody_html.append(f'<tr>{"".join(tr)}</tr>')

                html_blocks.append(
                    f'<div class="table-wrapper">'
                    f'<table>'
                    f'<thead><tr>{th_html}</tr></thead>'
                    f'<tbody>{"".join(tbody_html)}</tbody>'
                    f'</table>'
                    f'</div>'
                )
                continue

            # Lists: unordered (- or *) or ordered (1.)
            list_match = re.match(r"^(\s*)([-*]|\d+\.)\s+(.+)$", line)
            if list_match:
                list_items = []
                is_ordered = list_match.group(2).endswith(".")
                tag = "ol" if is_ordered else "ul"

                while i < n:
                    item_match = re.match(r"^(\s*)([-*]|\d+\.)\s+(.+)$", remaining_lines[i])
                    if not item_match:
                        # Check if line is indented continuation
                        if remaining_lines[i].startswith("   ") or remaining_lines[i].startswith("\t"):
                            if list_items:
                                list_items[-1] += "\n" + remaining_lines[i].strip()
                            i += 1
                            continue
                        break

                    list_items.append(item_match.group(3))
                    i += 1

                rendered_items = []
                for item_text in list_items:
                    rendered_items.append(f"<li>{self.parse_inline(item_text)}</li>")

                html_blocks.append(f'<{tag}>{"".join(rendered_items)}</{tag}>')
                continue

            # Regular Paragraph
            para_lines = [line]
            i += 1
            while i < n and remaining_lines[i].strip() and not remaining_lines[i].strip().startswith(("#", "```", ">", "|", "---", "* ", "- ")) and not re.match(r"^\d+\.\s+", remaining_lines[i]):
                para_lines.append(remaining_lines[i])
                i += 1

            para_text = " ".join(l.strip() for l in para_lines)
            html_blocks.append(f"<p>{self.parse_inline(para_text)}</p>")

        return "\n".join(html_blocks), self.toc


def build_docs_navigation(docs_meta: list[dict]) -> tuple[dict[str, list[dict]], list[dict]]:
    """Organize docs into categories and flat sequential list."""
    categorized = {cat: [] for cat in CATEGORY_ORDER}
    for doc in docs_meta:
        cat = doc.get("category", "General")
        if cat not in categorized:
            categorized[cat] = []
        categorized[cat].append(doc)

    for cat in categorized:
        categorized[cat].sort(key=lambda d: (d.get("order", 99), d.get("title", "")))

    flat_list = []
    for cat in CATEGORY_ORDER:
        flat_list.extend(categorized.get(cat, []))

    return categorized, flat_list


def generate_search_index(docs_meta: list[dict]) -> list[dict]:
    """Generate lightweight client-side search index."""
    index = []
    for doc in docs_meta:
        index.append({
            "title": doc["title"],
            "url": doc["filename"],
            "category": doc.get("category", ""),
            "description": doc.get("description", ""),
            "headings": [{"title": h["title"], "slug": h["slug"]} for h in doc.get("toc", [])],
        })
    return index


def render_doc_sidebar(categorized_nav: dict[str, list[dict]], current_filename: str) -> str:
    """Render HTML for docs sidebar navigation."""
    sections_html = []
    for cat in CATEGORY_ORDER:
        items = categorized_nav.get(cat, [])
        if not items:
            continue
        links = []
        for doc in items:
            is_active = doc["filename"] == current_filename
            active_class = ' class="active" aria-current="page"' if is_active else ""
            links.append(f'<li><a href="{doc["filename"]}"{active_class}>{html.escape(doc["title"])}</a></li>')

        sections_html.append(
            f'<div class="sidebar-group">'
            f'<div class="sidebar-heading">{html.escape(cat)}</div>'
            f'<ul class="sidebar-links">{"".join(links)}</ul>'
            f'</div>'
        )

    return "\n".join(sections_html)


def render_doc_toc(toc_items: list[dict]) -> str:
    """Render Table of Contents right-rail widget."""
    if not toc_items:
        return ""

    links = []
    for item in toc_items:
        level_class = "toc-l3" if item["level"] == 3 else "toc-l2"
        links.append(f'<li><a href="#{item["slug"]}" class="{level_class}">{html.escape(item["title"])}</a></li>')

    return (
        f'<nav class="toc-rail" aria-label="Table of contents">'
        f'<div class="toc-title">On this page</div>'
        f'<ul class="toc-list">{"".join(links)}</ul>'
        f'</nav>'
    )


def render_doc_page(
    doc: dict,
    body_html: str,
    categorized_nav: dict[str, list[dict]],
    prev_doc: dict | None,
    next_doc: dict | None,
) -> str:
    """Render complete HTML documentation page."""
    current_filename = doc["filename"]
    sidebar_html = render_doc_sidebar(categorized_nav, current_filename)
    toc_html = render_doc_toc(doc.get("toc", []))

    prev_html = ""
    if prev_doc:
        prev_html = (
            f'<a href="{prev_doc["filename"]}" class="doc-pager doc-pager-prev">'
            f'<span class="pager-hint">← Previous</span>'
            f'<span class="pager-title">{html.escape(prev_doc["title"])}</span>'
            f'</a>'
        )

    next_html = ""
    if next_doc:
        next_html = (
            f'<a href="{next_doc["filename"]}" class="doc-pager doc-pager-next">'
            f'<span class="pager-hint">Next →</span>'
            f'<span class="pager-title">{html.escape(next_doc["title"])}</span>'
            f'</a>'
        )

    page_url = f"{SITE_URL}/docs/{current_filename}" if current_filename != "index.html" else f"{SITE_URL}/docs/"
    page_title = f"{doc['title']} · agrep Docs"
    page_desc = doc.get('description', '') or f"Documentation and guide for {doc['title']} in agrep Arabic-script search tool."
    page_keywords = f"agrep, {doc['title'].lower()}, {doc.get('category', '').lower()}, arabic search, unicode normalization, cli, go, search engine"

    json_ld = {
        "@context": "https://schema.org",
        "@graph": [
            {
                "@type": "TechArticle",
                "headline": doc["title"],
                "description": page_desc,
                "url": page_url,
                "inLanguage": "en",
                "author": {
                    "@type": "Person",
                    "name": "Mohamed Elashri",
                    "url": "https://github.com/MohamedElashri",
                },
                "publisher": {
                    "@type": "Organization",
                    "name": "agrep",
                    "url": f"{SITE_URL}/",
                },
            },
            {
                "@type": "BreadcrumbList",
                "itemListElement": [
                    {
                        "@type": "ListItem",
                        "position": 1,
                        "name": "Home",
                        "item": f"{SITE_URL}/",
                    },
                    {
                        "@type": "ListItem",
                        "position": 2,
                        "name": "Docs",
                        "item": f"{SITE_URL}/docs/",
                    },
                    {
                        "@type": "ListItem",
                        "position": 3,
                        "name": doc.get("category", "Guide"),
                        "item": page_url,
                    },
                ],
            },
        ],
    }

    return f"""<!doctype html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{html.escape(page_title)}</title>
  <meta name="description" content="{html.escape(page_desc)}">
  <meta name="keywords" content="{html.escape(page_keywords)}">
  <meta name="author" content="Mohamed Elashri">
  <meta name="robots" content="index, follow">
  <link rel="canonical" href="{page_url}">
  <link rel="icon" type="image/svg+xml" href="../favicon.svg">
  <link rel="manifest" href="../site.webmanifest">
  <meta name="theme-color" content="#0c1a1b" media="(prefers-color-scheme: dark)">
  <meta name="theme-color" content="#f6f5ef" media="(prefers-color-scheme: light)">

  <!-- Open Graph / Social Sharing -->
  <meta property="og:site_name" content="agrep">
  <meta property="og:type" content="article">
  <meta property="og:title" content="{html.escape(page_title)}">
  <meta property="og:description" content="{html.escape(page_desc)}">
  <meta property="og:url" content="{page_url}">
  <meta property="og:image" content="{SITE_URL}/og-image.svg">
  <meta property="og:image:width" content="1200">
  <meta property="og:image:height" content="630">
  <meta property="og:image:alt" content="agrep documentation: {html.escape(doc['title'])}">
  <meta property="og:locale" content="en_US">

  <!-- Twitter Card -->
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:site" content="@MohamedElashri">
  <meta name="twitter:creator" content="@MohamedElashri">
  <meta name="twitter:title" content="{html.escape(page_title)}">
  <meta name="twitter:description" content="{html.escape(page_desc)}">
  <meta name="twitter:image" content="{SITE_URL}/og-image.svg">

  <!-- Structured Data (JSON-LD) -->
  <script type="application/ld+json">
{json.dumps(json_ld, indent=2)}
  </script>

  <link rel="stylesheet" href="../site.css">
  <link rel="stylesheet" href="docs.css">
</head>
<body class="docs-body">
  <header class="site-header">
    <div class="header-inner">
      <div class="header-left">
        <button id="sidebar-toggle" class="sidebar-toggle-btn" aria-label="Toggle Navigation Menu">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 6h16M4 12h16M4 18h16"/></svg>
        </button>
        <a href="../index.html" class="brand">
          <span class="brand-mark" aria-hidden="true">ا</span>
          <span class="brand-text">agrep<span class="brand-dot">.</span></span>
        </a>
        <span class="header-badge">docs</span>
      </div>
      <nav class="header-nav">
        <a href="../index.html">Home</a>
        <a href="index.html" class="active">Docs</a>
        <a href="../playground/index.html">Playground</a>
        <a href="benchmarks.html">Benchmarks</a>
        <a href="https://github.com/MohamedElashri/agrep" target="_blank" rel="noopener noreferrer" class="github-link">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
          <span>GitHub</span>
        </a>
        <button id="theme-toggle" class="theme-toggle-btn" aria-label="Toggle Dark/Light Mode">
          <svg class="sun-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
          <svg class="moon-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
        </button>
      </nav>
    </div>
  </header>

  <div class="docs-layout">
    <aside class="docs-sidebar" id="docs-sidebar">
      <div class="sidebar-search">
        <div class="search-input-wrap">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
          <input type="text" id="doc-filter" placeholder="Search docs... (Press /)" autocomplete="off">
        </div>
      </div>
      <div class="sidebar-content">
        {sidebar_html}
      </div>
    </aside>

    <main class="docs-main">
      <div class="docs-container">
        <nav class="breadcrumbs" aria-label="Breadcrumbs">
          <a href="../index.html">agrep</a>
          <span class="sep">/</span>
          <a href="index.html">Docs</a>
          <span class="sep">/</span>
          <span class="current">{html.escape(doc.get('category', 'Guide'))}</span>
        </nav>

        <article class="doc-content">
          <header class="doc-header">
            <p class="doc-category">{html.escape(doc.get('category', 'Documentation'))}</p>
            <h1>{html.escape(doc['title'])}</h1>
            {f'<p class="doc-lead">{html.escape(doc["description"])}</p>' if doc.get('description') else ''}
          </header>

          <div class="doc-body">
            {body_html}
          </div>

          <footer class="doc-footer">
            <div class="doc-pagers">
              <div class="pager-col">{prev_html}</div>
              <div class="pager-col">{next_html}</div>
            </div>
            <div class="doc-meta">
              <a href="https://github.com/MohamedElashri/agrep/blob/main/docs/{html.escape(doc.get('src_filename', ''))}" target="_blank" rel="noopener noreferrer">Edit this page on GitHub</a>
            </div>
          </footer>
        </article>
      </div>
    </main>

    <aside class="docs-toc">
      {toc_html}
    </aside>
  </div>

  <div id="search-modal" class="search-modal" aria-hidden="true" role="dialog" aria-modal="true">
    <div class="search-modal-backdrop"></div>
    <div class="search-modal-card">
      <div class="modal-search-bar">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
        <input type="text" id="modal-search-input" placeholder="Type to search documentation..." autocomplete="off">
        <kbd class="modal-search-esc">ESC</kbd>
      </div>
      <div id="modal-search-results" class="modal-search-results"></div>
    </div>
  </div>

  <script src="../site.js"></script>
  <script src="docs.js"></script>
</body>
</html>
"""


def render_landing_page() -> str:
    """Render the high-impact agrep Landing Page."""
    json_ld = {
        "@context": "https://schema.org",
        "@graph": [
            {
                "@type": "SoftwareApplication",
                "name": "agrep",
                "headline": "Ultrafast Unicode-Aware Search for Arabic-Script Text",
                "description": "agrep is an ultrafast, Unicode-aware search tool and Go library tailored for Arabic, Persian, Urdu, Pashto, Sorani Kurdish, and Uyghur text.",
                "applicationCategory": "DeveloperApplication",
                "applicationSubCategory": "Search CLI",
                "operatingSystem": "Linux, macOS, Windows, FreeBSD, OpenBSD, NetBSD",
                "softwareVersion": "0.1.1",
                "license": "https://opensource.org/licenses/MIT",
                "url": f"{SITE_URL}/",
                "codeRepository": "https://github.com/MohamedElashri/agrep",
                "author": {
                    "@type": "Person",
                    "name": "Mohamed Elashri",
                    "url": "https://github.com/MohamedElashri",
                },
                "offers": {
                    "@type": "Offer",
                    "price": "0",
                    "priceCurrency": "USD",
                },
            },
            {
                "@type": "WebSite",
                "name": "agrep",
                "url": f"{SITE_URL}/",
                "description": "Official landing page, documentation, and WebAssembly playground for agrep.",
            },
        ],
    }

    html_str = """<!doctype html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>agrep · Ultrafast Unicode-Aware Search for Arabic-Script Text</title>
  <meta name="description" content="agrep is a high-performance CLI search tool and Go library built specifically for Arabic, Persian, Urdu, Pashto, Kurdish, and Uyghur text. Matches across diacritics, ligatures, and consonant skeletons.">
  <meta name="keywords" content="__KEYWORDS__">
  <meta name="author" content="Mohamed Elashri">
  <meta name="robots" content="index, follow">
  <link rel="canonical" href="__SITE_URL__/">
  <link rel="icon" type="image/svg+xml" href="favicon.svg">
  <link rel="manifest" href="site.webmanifest">
  <meta name="theme-color" content="#0c1a1b" media="(prefers-color-scheme: dark)">
  <meta name="theme-color" content="#f6f5ef" media="(prefers-color-scheme: light)">

  <!-- Open Graph / Social Sharing -->
  <meta property="og:site_name" content="agrep">
  <meta property="og:type" content="website">
  <meta property="og:title" content="agrep · Ultrafast Unicode-Aware Search for Arabic-Script Text">
  <meta property="og:description" content="Ultrafast, Unicode-aware search tool tailored for Arabic-script text. Matches across diacritics, letter variants, ligatures, and dotless rasm with exact source byte mapping.">
  <meta property="og:url" content="__SITE_URL__/">
  <meta property="og:image" content="__SITE_URL__/og-image.svg">
  <meta property="og:image:width" content="1200">
  <meta property="og:image:height" content="630">
  <meta property="og:image:alt" content="agrep - Ultrafast Arabic Search Tool">
  <meta property="og:locale" content="en_US">

  <!-- Twitter Card -->
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:site" content="@MohamedElashri">
  <meta name="twitter:creator" content="@MohamedElashri">
  <meta name="twitter:title" content="agrep · Ultrafast Unicode-Aware Search for Arabic-Script Text">
  <meta name="twitter:description" content="Ultrafast, Unicode-aware search tool tailored for Arabic-script text. Matches across diacritics, letter variants, ligatures, and dotless rasm with exact source byte mapping.">
  <meta name="twitter:image" content="__SITE_URL__/og-image.svg">

  <!-- Structured Data (JSON-LD) -->
  <script type="application/ld+json">
__JSON_LD__
  </script>

  <link rel="stylesheet" href="site.css">
  <link rel="stylesheet" href="landing.css">
</head>
<body class="landing-body">
  <header class="site-header">
    <div class="header-inner">
      <div class="header-left">
        <a href="index.html" class="brand">
          <span class="brand-mark" aria-hidden="true">ا</span>
          <span class="brand-text">agrep<span class="brand-dot">.</span></span>
        </a>
      </div>
      <nav class="header-nav">
        <a href="docs/index.html">Docs</a>
        <a href="playground/index.html">Playground</a>
        <a href="docs/benchmarks.html">Benchmarks</a>
        <a href="https://github.com/MohamedElashri/agrep" target="_blank" rel="noopener noreferrer" class="github-link">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
          <span>GitHub</span>
        </a>
        <button id="theme-toggle" class="theme-toggle-btn" aria-label="Toggle Dark/Light Mode">
          <svg class="sun-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
          <svg class="moon-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
        </button>
      </nav>
    </div>
  </header>

  <!-- Hero Section -->
  <section class="hero-section">
    <div class="hero-bg-accent" aria-hidden="true"></div>
    <div class="hero-container">
      <div class="hero-badge">
        <span class="badge-dot"></span>
        <span>v0.1.1 Released · WebAssembly Live Demo Included</span>
      </div>

      <h1 class="hero-title">
        Ultrafast, Unicode-aware search for <span class="highlight">Arabic-script</span> text.
      </h1>

      <p class="hero-subtitle">
        Standard tools like <code>grep</code> and <code>ripgrep</code> miss matches due to diacritics, cursive ligatures, letter variants, and diverse orthographies. <strong>agrep</strong> eliminates search blind spots with zero-allocation streaming normalization, dotless rasm, and exact source byte mapping.
      </p>

      <div class="hero-actions">
        <a href="docs/index.html" class="btn btn-primary">
          <span>Explore Documentation</span>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14M12 5l7 7-7 7"/></svg>
        </a>
        <a href="playground/index.html" class="btn btn-secondary">
          <span class="live-indicator"></span>
          <span>Try Web Playground</span>
        </a>
        <a href="https://github.com/MohamedElashri/agrep" target="_blank" rel="noopener noreferrer" class="btn btn-ghost">
          <span>Star on GitHub</span>
        </a>
      </div>

      <!-- Quick Install Box -->
      <div class="hero-install-card">
        <div class="install-tabs">
          <button class="install-tab active" data-install="curl">curl install</button>
          <button class="install-tab" data-install="go">go install</button>
          <button class="install-tab" data-install="binary">binary release</button>
        </div>
        <div class="install-command-wrap">
          <code id="install-command">curl -fsSL https://raw.githubusercontent.com/MohamedElashri/agrep/main/scripts/install.sh | bash</code>
          <button id="copy-install-btn" class="copy-btn" aria-label="Copy install command">
            <svg class="copy-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
            <span class="copy-text">Copy</span>
          </button>
        </div>
      </div>
    </div>
  </section>

  <!-- Interactive Demo Showcase -->
  <section class="demo-section">
    <div class="section-container">
      <div class="section-header">
        <p class="section-eyebrow">Interactive Comparison</p>
        <h2>See why generic search fails on Arabic</h2>
        <p class="section-desc">Click through the scenarios below to see how agrep handles the complex orthographic realities of Arabic-script text while standard grep fails.</p>
      </div>

      <div class="demo-widget">
        <div class="demo-nav" role="tablist">
          <button class="demo-tab active" data-scenario="0" role="tab">Tashkil (Vowels)</button>
          <button class="demo-tab" data-scenario="1" role="tab">PDF Ligatures</button>
          <button class="demo-tab" data-scenario="2" role="tab">Cross-Language (Persian)</button>
          <button class="demo-tab" data-scenario="3" role="tab">Dotless Rasm (Manuscripts)</button>
          <button class="demo-tab" data-scenario="4" role="tab">Fuzzy Levenshtein</button>
        </div>

        <div class="demo-display">
          <div class="demo-col">
            <div class="demo-box-label">Standard grep / ripgrep</div>
            <div class="demo-box demo-box-fail" id="grep-box">
              <div class="demo-cli-cmd">$ grep 'مدرسه' corpus.txt</div>
              <div class="demo-result-text" dir="auto">(No matches found)</div>
              <div class="demo-status-pill pill-fail">❌ 0 matches (missed text)</div>
            </div>
          </div>

          <div class="demo-col">
            <div class="demo-box-label">agrep (Arabic-script aware)</div>
            <div class="demo-box demo-box-success" id="agrep-box">
              <div class="demo-cli-cmd">$ agrep 'مدرسه' corpus.txt</div>
              <div class="demo-result-text" dir="auto" id="agrep-match-text">هذه <mark>مَدْرَسَةٌ</mark> عريقة في المدينة.</div>
              <div class="demo-status-pill pill-success">✅ Match found with exact byte spans</div>
            </div>
          </div>
        </div>

        <div class="demo-explainer" id="demo-explanation">
          <strong>How it works:</strong> The query <code>مدرسه</code> automatically folds ta-marbuta (<code>ة</code> → <code>ه</code>) and strips vocalization diacritics (<em>fathah, dammah, tanwin</em>) during normalized key comparison, while preserving original source text and exact UTF-8 byte spans.
        </div>
      </div>
    </div>
  </section>

  <!-- Key Capabilities Grid -->
  <section class="features-section">
    <div class="section-container">
      <div class="section-header">
        <p class="section-eyebrow">Engineered for Performance & Accuracy</p>
        <h2>Built for researchers, engineers, and AI agents</h2>
        <p class="section-desc">From 100-gigabyte historical corpora to real-time agentic codebases, agrep is fast, correct, and modular.</p>
      </div>

      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
          </div>
          <h3>Streaming Unicode Normalization</h3>
          <p>Gigabyte-per-second Unicode NFD pipeline with 5 standard presets (<code>search</code>, <code>strict</code>, <code>loose</code>, <code>lucene</code>, <code>camel</code>) and granular flags to keep or fold specific marks.</p>
          <a href="docs/normalization.html" class="feature-link">Read Normalization Docs →</a>
        </div>

        <div class="feature-card">
          <div class="feature-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
          </div>
          <h3>Six Arabic-Script Languages</h3>
          <p>First-class support for Persian (Farsi), Urdu, Pashto, Sorani Kurdish, and Uyghur. Retains distinct language alphabets or enables shared letter search across Arabic and Persian.</p>
          <a href="docs/languages.html" class="feature-link">Read Language Support →</a>
        </div>

        <div class="feature-card">
          <div class="feature-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>
          </div>
          <h3>Dotless Rasm & Myers Fuzzy</h3>
          <p>Search unpointed historical manuscripts where <code>ب ت ث ن ي</code> share the dotless skeleton <code>ٮ</code>. Bit-parallel Myers algorithm handles typos, dialect variance, and OCR errors.</p>
          <a href="docs/matching.html" class="feature-link">Read Matching Docs →</a>
        </div>

        <div class="feature-card">
          <div class="feature-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
          </div>
          <h3>Exact Source-Byte Spans</h3>
          <p>Matches on normalized keys, but maps every span back to original text UTF-8 byte offsets. Even expanded presentation forms like <code>ﻻ</code> map back to the single source character.</p>
          <a href="docs/cli.html" class="feature-link">Explore JSON Output →</a>
        </div>

        <div class="feature-card">
          <div class="feature-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
          </div>
          <h3>Legacy Encodings & Transliteration</h3>
          <p>Automatic detection for CP1256, ISO-8859-6, and UTF-16. Search Arabic corpora from a Latin keyboard using standard Buckwalter, ArabTeX, or ISO-233 transliteration schemes.</p>
          <a href="docs/input.html" class="feature-link">Read Encodings Guide →</a>
        </div>

        <div class="feature-card">
          <div class="feature-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>
          </div>
          <h3>Modular Go Packages & Agent Skills</h3>
          <p>Use agrep as a standalone CLI or import modular Go packages (<code>arabic</code>, <code>match</code>, <code>scan</code>). Ships with pre-built skills for Claude Code and OpenAI Codex harnesses.</p>
          <a href="docs/development.html" class="feature-link">View Agent Skills →</a>
        </div>
      </div>
    </div>
  </section>

  <!-- Performance Benchmark Highlight -->
  <section class="benchmark-section">
    <div class="section-container">
      <div class="benchmark-card">
        <div class="benchmark-content">
          <p class="section-eyebrow">High Throughput</p>
          <h2>Over 1,200 MiB/s throughput on multi-threaded search</h2>
          <p>
            agrep is built with Go and optimized with SIMD and zero-allocation hot paths. It provides the linguistic sophistication of specialized NLP packages while retaining the streaming speed expected of modern system utilities.
          </p>
          <div class="benchmark-stats">
            <div class="stat-item">
              <span class="stat-number">1,214 <span class="stat-unit">MiB/s</span></span>
              <span class="stat-label">Recursive search throughput</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">&lt; 1 <span class="stat-unit">ms</span></span>
              <span class="stat-label">CLI startup latency</span>
            </div>
            <div class="stat-item">
              <span class="stat-number">6 <span class="stat-unit">Alphabets</span></span>
              <span class="stat-label">Full language orthographies</span>
            </div>
          </div>
          <div class="benchmark-cta">
            <a href="docs/benchmarks.html" class="btn btn-secondary">View Complete Benchmark Data →</a>
          </div>
        </div>
      </div>
    </div>
  </section>

  <!-- Go Package Code Snippet Section -->
  <section class="code-section">
    <div class="section-container">
      <div class="section-header">
        <p class="section-eyebrow">Embeddable Library</p>
        <h2>Use agrep as a modular Go library</h2>
        <p class="section-desc">Integrate Unicode-aware Arabic normalization and matching directly into your Go services and pipelines.</p>
      </div>

      <div class="code-showcase">
        <div class="code-bar">
          <span class="code-lang">Go</span>
          <button class="copy-btn" data-copy-target="go-snippet">Copy</button>
        </div>
        <pre><code id="go-snippet" class="language-go">package main

import (
    "fmt"
    "github.com/MohamedElashri/agrep/arabic"
    "github.com/MohamedElashri/agrep/match"
)

func main() {
    // Normalization collapses diacritics and spelling variants
    p := arabic.ProfileSearch
    key1 := p.Normalize("مَدْرَسَةٌ")
    key2 := p.Normalize("مدرسه")
    fmt.Println(key1 == key2) // true

    // High-performance Myers fuzzy search
    m, _ := match.NewFuzzy("كتاب", 1, p)
    matches := m.FindAll("هذا كتلب جديد", -1)
    fmt.Printf("Matched %d spans\n", len(matches))
}</code></pre>
      </div>
    </div>
  </section>

  <!-- Footer -->
  <footer class="site-footer">
    <div class="footer-inner">
      <div class="footer-brand-col">
        <a href="index.html" class="brand">
          <span class="brand-mark" aria-hidden="true">ا</span>
          <span class="brand-text">agrep<span class="brand-dot">.</span></span>
        </a>
        <p class="footer-tagline">Ultrafast Unicode-aware search tool tailored for Arabic-script text and corpora.</p>
        <p class="footer-copyright">Released under the MIT License.<br>© Mohamed Elashri and agrep contributors.</p>
      </div>

      <div class="footer-links-col">
        <h4>Documentation</h4>
        <ul>
          <li><a href="docs/index.html">Overview & Quickstart</a></li>
          <li><a href="docs/cli.html">Command-Line Reference</a></li>
          <li><a href="docs/normalization.html">Unicode Normalization</a></li>
          <li><a href="docs/languages.html">Language Orthographies</a></li>
          <li><a href="docs/matching.html">Rasm & Fuzzy Search</a></li>
          <li><a href="docs/input.html">Encodings & Transliteration</a></li>
        </ul>
      </div>

      <div class="footer-links-col">
        <h4>Ecosystem</h4>
        <ul>
          <li><a href="playground/index.html">Normalization Playground</a></li>
          <li><a href="docs/benchmarks.html">Performance Benchmarks</a></li>
          <li><a href="docs/terminals.html">Terminal Color & BiDi</a></li>
          <li><a href="docs/development.html">Agent Integration (Codex/Claude)</a></li>
          <li><a href="docs/distribution.html">Release Checklist</a></li>
        </ul>
      </div>

      <div class="footer-links-col">
        <h4>Community</h4>
        <ul>
          <li><a href="https://github.com/MohamedElashri/agrep" target="_blank" rel="noopener noreferrer">GitHub Repository</a></li>
          <li><a href="https://github.com/MohamedElashri/agrep/releases" target="_blank" rel="noopener noreferrer">Release Downloads</a></li>
          <li><a href="https://pkg.go.dev/github.com/MohamedElashri/agrep" target="_blank" rel="noopener noreferrer">Go Reference (pkg.go.dev)</a></li>
          <li><a href="https://github.com/MohamedElashri/agrep/issues" target="_blank" rel="noopener noreferrer">Issue Tracker</a></li>
        </ul>
      </div>
    </div>
  </footer>

  <script src="site.js"></script>
  <script src="landing.js"></script>
</body>
</html>
"""
    return (
        html_str.replace("__KEYWORDS__", DEFAULT_KEYWORDS)
        .replace("__SITE_URL__", SITE_URL)
        .replace("__JSON_LD__", json.dumps(json_ld, indent=2))
    )


def render_playground_page() -> str:
    """Render the playground page with unified site branding and navigation."""
    json_ld = {
        "@context": "https://schema.org",
        "@type": "WebApplication",
        "name": "agrep Normalization Playground",
        "url": f"{SITE_URL}/playground/",
        "applicationCategory": "DeveloperApplication",
        "operatingSystem": "All (WebAssembly)",
        "description": "Interactive WebAssembly browser playground for testing Arabic Unicode normalization rules and comparison keys in real-time.",
    }

    html_str = """<!doctype html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Normalization Playground · agrep</title>
  <meta name="description" content="Interactive browser playground for agrep. Test Unicode normalization profiles, Arabic presentation forms, and comparison keys in real-time via WebAssembly.">
  <meta name="keywords" content="agrep, playground, webassembly, arabic normalization, unicode, live demo, search engine">
  <meta name="author" content="Mohamed Elashri">
  <meta name="robots" content="index, follow">
  <link rel="canonical" href="__SITE_URL__/playground/">
  <link rel="icon" type="image/svg+xml" href="../favicon.svg">
  <link rel="manifest" href="../site.webmanifest">
  <meta name="theme-color" content="#0c1a1b" media="(prefers-color-scheme: dark)">
  <meta name="theme-color" content="#f6f5ef" media="(prefers-color-scheme: light)">

  <!-- Open Graph / Social Sharing -->
  <meta property="og:site_name" content="agrep">
  <meta property="og:type" content="website">
  <meta property="og:title" content="Normalization Playground · agrep">
  <meta property="og:description" content="Interactive browser playground for agrep. Test Unicode normalization profiles, Arabic presentation forms, and comparison keys in real-time via WebAssembly.">
  <meta property="og:url" content="__SITE_URL__/playground/">
  <meta property="og:image" content="__SITE_URL__/og-image.svg">
  <meta property="og:image:width" content="1200">
  <meta property="og:image:height" content="630">
  <meta property="og:image:alt" content="agrep Normalization Playground">
  <meta property="og:locale" content="en_US">

  <!-- Twitter Card -->
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:site" content="@MohamedElashri">
  <meta name="twitter:creator" content="@MohamedElashri">
  <meta name="twitter:title" content="Normalization Playground · agrep">
  <meta name="twitter:description" content="Interactive browser playground for agrep. Test Unicode normalization profiles, Arabic presentation forms, and comparison keys in real-time via WebAssembly.">
  <meta name="twitter:image" content="__SITE_URL__/og-image.svg">

  <!-- Structured Data (JSON-LD) -->
  <script type="application/ld+json">
__JSON_LD__
  </script>

  <link rel="stylesheet" href="../site.css">
  <link rel="stylesheet" href="style.css">
  <script src="wasm_exec.js" defer></script>
  <script src="app.js" defer></script>
</head>
<body class="playground-body">
  <header class="site-header">
    <div class="header-inner">
      <div class="header-left">
        <a href="../index.html" class="brand">
          <span class="brand-mark" aria-hidden="true">ا</span>
          <span class="brand-text">agrep<span class="brand-dot">.</span></span>
        </a>
        <span class="header-badge">playground</span>
      </div>
      <nav class="header-nav">
        <a href="../index.html">Home</a>
        <a href="../docs/index.html">Docs</a>
        <a href="index.html" class="active">Playground</a>
        <a href="../docs/benchmarks.html">Benchmarks</a>
        <a href="https://github.com/MohamedElashri/agrep" target="_blank" rel="noopener noreferrer" class="github-link">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
          <span>GitHub</span>
        </a>
        <button id="theme-toggle" class="theme-toggle-btn" aria-label="Toggle Dark/Light Mode">
          <svg class="sun-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
          <svg class="moon-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
        </button>
      </nav>
    </div>
  </header>

  <main class="shell">
    <aside class="intro" aria-labelledby="page-title">
      <div class="brand"><span class="brand-mark" aria-hidden="true">ا</span><span>agrep<span class="brand-dot">.</span></span></div>
      <div class="intro-copy">
        <p class="eyebrow">Arabic-script search / playground</p>
        <h1 id="page-title">See the text behind the match.</h1>
        <p>See the comparison key agrep uses for searching. Switch profiles and languages to explore the result.</p>
      </div>
      <div class="intro-foot">
        <span class="intro-line" aria-hidden="true"></span>
        <p>Same Go normalization library as the CLI. Runs locally in your browser via WebAssembly.</p>
      </div>
    </aside>

    <div class="main-column">
      <header class="page-header">
        <div>
          <p class="section-kicker">Interactive WebAssembly tool</p>
          <h2>Normalization playground</h2>
        </div>
        <span class="live-label"><span aria-hidden="true"></span> Live preview</span>
      </header>

      <section class="workspace" aria-label="Normalization playground">
        <div class="controls">
          <label for="profile">Normalization profile
            <select id="profile">
              <option value="search">Search (Default)</option>
              <option value="strict">Strict</option>
              <option value="loose">Loose (High Recall)</option>
              <option value="lucene">Lucene Compatible</option>
              <option value="camel">CAMeL Compatible</option>
            </select>
          </label>
          <label for="languages">Text language
            <select id="languages">
              <option value="ar">Arabic</option>
              <option value="fa">Persian</option>
              <option value="ar,fa">Arabic + Persian</option>
              <option value="ur">Urdu</option>
              <option value="ps">Pashto</option>
              <option value="ku">Sorani Kurdish</option>
              <option value="ug">Uyghur</option>
            </select>
          </label>
        </div>

        <div class="examples" aria-label="Example texts">
          <span>Try an example</span>
          <div class="example-list">
            <button type="button" data-example="أعلنت المدينة افتتاح مكتبة جديدة." data-profile="search" data-languages="ar">Spelling folds</button>
            <button type="button" data-example="ﻻ يوجد ﻛِﺘـﺎﺏ" data-profile="search" data-languages="ar">PDF forms</button>
            <button type="button" data-example="کتاب تازه است." data-profile="search" data-languages="ar,fa">Persian</button>
            <button type="button" data-example="ب ت ث ن ي" data-profile="loose" data-languages="ar">Dotless rasm</button>
          </div>
        </div>

        <div class="panels">
          <div class="panel panel-input">
            <div class="panel-top"><label for="input"><span class="panel-number">01</span> Original text</label><button class="text-button" id="clear" type="button">Clear</button></div>
            <textarea id="input" dir="auto" spellcheck="false" placeholder="Type or paste Arabic-script text…">ﻻ يوجد ﻛِﺘـﺎﺏ</textarea>
          </div>
          <div class="panel panel-result">
            <div class="panel-top"><span class="panel-title"><span class="panel-number">02</span> Comparison key</span><button class="text-button" id="copy" type="button" disabled>Copy key</button></div>
            <output id="key" class="key" dir="auto" aria-live="polite">Loading…</output>
            <p class="result-note">This is the key agrep compares during matching.</p>
          </div>
        </div>

        <div class="rule-panel">
          <h3>Rules affecting this text</h3>
          <p>A rule appears when turning it off alone would change the key.</p>
          <div id="rules" class="rules" aria-live="polite"></div>
        </div>
        <p id="status" class="status" role="status">Loading the Go WebAssembly module…</p>
      </section>

      <footer>Read the <a href="../docs/normalization.html">normalization reference</a> for rule definitions and caveats. <a href="GO-LICENSE.txt">Go runtime license</a>.</footer>
    </div>
  </main>
  <script src="../site.js"></script>
</body>
</html>
"""
    return (
        html_str.replace("__SITE_URL__", SITE_URL)
        .replace("__JSON_LD__", json.dumps(json_ld, indent=2))
    )


def build_site(repo_root: Path, output_dir: Path):
    """Build the entire static site."""
    docs_dir = repo_root / "docs"
    web_dir = repo_root / "web"
    output_dir.mkdir(parents=True, exist_ok=True)
    docs_out = output_dir / "docs"
    docs_out.mkdir(parents=True, exist_ok=True)
    playground_out = output_dir / "playground"
    playground_out.mkdir(parents=True, exist_ok=True)

    # 1. Parse all Markdown documents in docs/
    docs_meta = []
    doc_bodies = {}

    for md_file in docs_dir.glob("*.md"):
        content = md_file.read_text(encoding="utf-8")
        meta, body = parse_frontmatter(content)
        base_name = md_file.name.lower()
        html_name = DOC_FILE_MAP.get(base_name, base_name.replace(".md", ".html"))

        parser = MarkdownParser(base_name)
        body_html, toc = parser.parse(body)

        title = meta.get("title", md_file.stem.replace("_", " ").title())
        doc_info = {
            "title": title,
            "description": meta.get("description", ""),
            "category": meta.get("category", "General"),
            "order": meta.get("order", 99),
            "filename": html_name,
            "src_filename": md_file.name,
            "toc": toc,
        }
        docs_meta.append(doc_info)
        doc_bodies[html_name] = body_html

    # Organize navigation
    categorized_nav, flat_list = build_docs_navigation(docs_meta)

    # 2. Render each documentation page
    for idx, doc in enumerate(flat_list):
        prev_doc = flat_list[idx - 1] if idx > 0 else None
        next_doc = flat_list[idx + 1] if idx < len(flat_list) - 1 else None
        html_content = render_doc_page(
            doc=doc,
            body_html=doc_bodies[doc["filename"]],
            categorized_nav=categorized_nav,
            prev_doc=prev_doc,
            next_doc=next_doc,
        )
        (docs_out / doc["filename"]).write_text(html_content, encoding="utf-8")

    # 3. Write search index
    search_index = generate_search_index(docs_meta)
    (docs_out / "search-index.json").write_text(json.dumps(search_index, ensure_ascii=False, indent=2), encoding="utf-8")

    # 4. Render Landing Page
    landing_html = render_landing_page()
    (output_dir / "index.html").write_text(landing_html, encoding="utf-8")

    # 5. Render Playground Page
    playground_html = render_playground_page()
    (playground_out / "index.html").write_text(playground_html, encoding="utf-8")

    # Also write a playground redirect at root: playground.html -> playground/index.html
    playground_redirect = '<!doctype html><html><head><meta http-equiv="refresh" content="0; url=playground/index.html"><title>Redirecting...</title></head><body><p>Redirecting to <a href="playground/index.html">playground</a>...</p></body></html>'
    (output_dir / "playground.html").write_text(playground_redirect, encoding="utf-8")

    # 6. Generate sitemap.xml and robots.txt
    (output_dir / "sitemap.xml").write_text(generate_sitemap(flat_list), encoding="utf-8")
    (output_dir / "robots.txt").write_text(generate_robots_txt(), encoding="utf-8")

    # 7. Copy assets from web/
    for item in ["site.css", "landing.css", "landing.js", "site.js", "favicon.svg", "og-image.svg", "site.webmanifest"]:
        src = web_dir / item
        if src.exists():
            shutil.copy2(src, output_dir / item)

    for item in ["docs.css", "docs.js", "favicon.svg", "site.webmanifest"]:
        src = web_dir / item
        if src.exists():
            shutil.copy2(src, docs_out / item)

    for item in ["app.js", "style.css", "GO-LICENSE.txt", "favicon.svg", "site.webmanifest"]:
        src = web_dir / item
        if src.exists():
            shutil.copy2(src, playground_out / item)
            # Also copy to root for legacy/smoke test compatibility
            shutil.copy2(src, output_dir / item)

    print(f"Successfully generated agrep site at: {output_dir}")
    print(f"Generated {len(flat_list)} documentation pages in {docs_out}")
    print(f"Generated landing page, playground, sitemap.xml, and robots.txt at {output_dir}")


def main():
    parser = argparse.ArgumentParser(description="Build agrep website, docs, and playground")
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parent.parent)
    parser.add_argument("--output-dir", type=Path, default=None)
    args = parser.parse_args()

    repo_root = args.repo_root
    output_dir = args.output_dir or (repo_root / "web" / "dist")
    build_site(repo_root, output_dir)


if __name__ == "__main__":
    main()
