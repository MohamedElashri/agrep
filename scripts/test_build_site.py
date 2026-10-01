import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

# Add scripts directory to path to import build-site functions directly
SCRIPTS_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPTS_DIR.parent
sys.path.insert(0, str(SCRIPTS_DIR))

import importlib.util
spec = importlib.util.spec_from_file_location("build_site_mod", SCRIPTS_DIR / "build-site.py")
build_site_mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build_site_mod)


class BuildSiteTest(unittest.TestCase):
    def test_parse_frontmatter(self):
        text = """---
title: "Sample Doc"
category: "Core"
order: 3
---

# Real Content
Here is the body.
"""
        meta, body = build_site_mod.parse_frontmatter(text)
        self.assertEqual(meta["title"], "Sample Doc")
        self.assertEqual(meta["category"], "Core")
        self.assertEqual(meta["order"], 3)
        self.assertIn("# Real Content", body)

    def test_slugify(self):
        self.assertEqual(build_site_mod.slugify("Unicode Normalization"), "unicode-normalization")
        self.assertEqual(build_site_mod.slugify("What's New in v0.1.0?!"), "whats-new-in-v010")
        self.assertEqual(build_site_mod.slugify("   Spaces and _ Underscores  "), "spaces-and-underscores")

    def test_markdown_parser_inline_and_links(self):
        parser = build_site_mod.MarkdownParser("cli.md")
        md = "See [Normalization](NORMALIZATION.md) and `code` with **bold**."
        parsed_html, _ = parser.parse(md)
        self.assertIn('<a href="normalization.html">Normalization</a>', parsed_html)
        self.assertIn("<code>code</code>", parsed_html)
        self.assertIn("<strong>bold</strong>", parsed_html)

    def test_markdown_parser_tables_and_callouts(self):
        parser = build_site_mod.MarkdownParser("test.md")
        md = """
> [!NOTE]
> This is a helpful note.

| Feature | Status |
| :--- | ---: |
| Fast | Yes |
"""
        parsed_html, _ = parser.parse(md)
        self.assertIn('class="callout callout-note"', parsed_html)
        self.assertIn("This is a helpful note.", parsed_html)
        self.assertIn("<table>", parsed_html)
        self.assertIn('<th class="text-left">Feature</th>', parsed_html)
        self.assertIn('<td class="text-right">Yes</td>', parsed_html)

    def test_full_build_site_execution(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            out_dir = Path(tmpdir)
            build_site_mod.build_site(REPO_ROOT, out_dir)

            # Check Landing Page
            landing_file = out_dir / "index.html"
            self.assertTrue(landing_file.exists())
            landing_content = landing_file.read_text(encoding="utf-8")
            self.assertIn("agrep", landing_content)
            self.assertIn("Ultrafast, Unicode-aware search", landing_content)
            self.assertIn("docs/index.html", landing_content)
            self.assertIn("playground/index.html", landing_content)

            # Check Documentation Pages
            docs_dir = out_dir / "docs"
            self.assertTrue(docs_dir.exists())
            doc_index = docs_dir / "index.html"
            self.assertTrue(doc_index.exists())
            cli_doc = docs_dir / "cli.html"
            self.assertTrue(cli_doc.exists())
            norm_doc = docs_dir / "normalization.html"
            self.assertTrue(norm_doc.exists())

            # Check Search Index
            search_index_file = docs_dir / "search-index.json"
            self.assertTrue(search_index_file.exists())
            index_data = json.loads(search_index_file.read_text(encoding="utf-8"))
            self.assertIsInstance(index_data, list)
            self.assertGreaterEqual(len(index_data), 10)
            titles = [item["title"] for item in index_data]
            self.assertIn("Command-Line Reference", titles)
            self.assertIn("Unicode Normalization Reference", titles)

            # Check Playground
            playground_dir = out_dir / "playground"
            self.assertTrue(playground_dir.exists())
            playground_index = playground_dir / "index.html"
            self.assertTrue(playground_index.exists())
            playground_content = playground_index.read_text(encoding="utf-8")
            self.assertIn('id="input"', playground_content)
            self.assertIn('id="profile"', playground_content)
            self.assertIn('id="key"', playground_content)

            # Check SEO assets and meta tags
            self.assertTrue((out_dir / "sitemap.xml").exists())
            sitemap_xml = (out_dir / "sitemap.xml").read_text(encoding="utf-8")
            self.assertIn("<urlset", sitemap_xml)
            self.assertIn("https://mohamedelashri.github.io/agrep/", sitemap_xml)
            self.assertIn("https://mohamedelashri.github.io/agrep/docs/cli.html", sitemap_xml)

            self.assertTrue((out_dir / "robots.txt").exists())
            robots_txt = (out_dir / "robots.txt").read_text(encoding="utf-8")
            self.assertIn("Sitemap: https://mohamedelashri.github.io/agrep/sitemap.xml", robots_txt)

            self.assertTrue((out_dir / "favicon.svg").exists())
            self.assertTrue((out_dir / "og-image.svg").exists())
            self.assertTrue((out_dir / "site.webmanifest").exists())

            # Check SEO in Landing HTML
            self.assertIn('property="og:site_name"', landing_content)
            self.assertIn('name="twitter:card"', landing_content)
            self.assertIn('rel="canonical"', landing_content)
            self.assertIn('application/ld+json', landing_content)

            # Check SEO in Doc HTML
            norm_content = norm_doc.read_text(encoding="utf-8")
            self.assertIn('property="og:type" content="article"', norm_content)
            self.assertIn('application/ld+json', norm_content)
            self.assertIn('BreadcrumbList', norm_content)


if __name__ == "__main__":
    unittest.main()
