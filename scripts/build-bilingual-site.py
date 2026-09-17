#!/usr/bin/env python3
"""Builds a real German output (in place) and a real English output
(site/en/) for each hand-written bilingual marketing page in site/,
baking each element's data-de/data-en value in as static content instead
of leaving it for script.js to inject at runtime. Two separate,
crawlable, hreflang-linked URLs beat one URL whose language depends on
JavaScript running - the same problem docs-site/ already solves via
mkdocs-static-i18n; this is the stdlib-only equivalent for the 5
hand-authored marketing pages, which have no build tooling of their own.

Run as a pages.yml step against the checked-out working copy, before the
Pages artifact is uploaded. It never touches the committed sources in a
way that survives the build - re-run it and it always starts fresh from
site/*.html, which stays dual-language (data-de/data-en) exactly as
authors edit it today. site/en/ is gitignored build output, like
site/docs/.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

SITE_DIR = Path(__file__).resolve().parent.parent / "site"
BASE_URL = "https://rootguard.foxly.de"
PAGES = ["index.html", "roadmap.html", "tools.html", "privacy.html", "imprint.html"]

# A real HTML "start tag" grammar - critically, an attribute's quoted
# value can itself contain a literal ">" (e.g. data-de="...<br><em>...")
# without ending the tag early. Four real headlines on this site use
# exactly that shape; a naive "<tag[^>]*>" breaks on them.
_ATTR = r'''(?:\s+[a-zA-Z_:][-a-zA-Z0-9_:.]*(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?)'''
OPEN_TAG_RE = re.compile(rf'<([a-zA-Z][a-zA-Z0-9]*){_ATTR}*\s*/?>')
DATA_DE_RE = re.compile(r'\bdata-de="([^"]*)"')
DATA_EN_RE = re.compile(r'\bdata-en="([^"]*)"')
STRIP_DATA_DE_EN_RE = re.compile(r'\s+data-(?:de|en)="[^"]*"')
BODY_DATA_ATTR_RE = re.compile(r'\s+data-(?:title|description|og-title|og-description)-(?:de|en)="[^"]*"')
LANG_SWITCH_RE = re.compile(
    r'<div class="language-switch nav-language" aria-label="Sprache">.*?</div>', re.DOTALL
)

# Shared files that stay at the site root rather than being duplicated
# per language - a page moved under site/en/ reaches them via a
# root-relative path instead of a recomputed "../" depth. Cross-page
# links (roadmap.html, index.html, ...) and anchors are deliberately not
# rewritten: a plain relative link to a sibling page must keep resolving
# to *that page's own* language copy, not jump back to the other one.
ASSET_NAMES = ("styles.css", "project.css", "legal.css", "rootguard-icon.svg", "script.js")
ASSET_HREF_RE = re.compile(r'(href|src)="(' + "|".join(re.escape(n) for n in ASSET_NAMES) + r')"')
ASSET_DIR_HREF_RE = re.compile(r'(href|src)="(assets/[^"]*)"')
DOCS_HREF_RE = re.compile(r'(href|src)="docs/([^"]*)"')


def find_matching_close(html: str, tag: str, search_from: int) -> tuple[int, int]:
    """Depth-counts same-named tags to find the element opened just before
    search_from. Returns (content_end, close_tag_end): content_end is
    where the matching "</tag>" begins, close_tag_end right after it."""
    depth = 1
    pos = search_from
    open_re = re.compile(rf'<{re.escape(tag)}\b')
    close_re = re.compile(rf'</{re.escape(tag)}>')
    while True:
        next_open = open_re.search(html, pos)
        next_close = close_re.search(html, pos)
        if next_close is None:
            raise ValueError(f"unclosed <{tag}> starting search at {search_from}")
        if next_open is not None and next_open.start() < next_close.start():
            depth += 1
            pos = next_open.end()
        else:
            depth -= 1
            if depth == 0:
                return next_close.start(), next_close.end()
            pos = next_close.end()


def bake_language(html: str, lang: str) -> str:
    """Replaces every element carrying both data-de and data-en with its
    lang-specific value as real content, then drops both now-redundant
    attributes - the same transform script.js's old setLanguage() used to
    do at runtime, done once here at build time instead."""
    out = []
    pos = 0
    for m in OPEN_TAG_RE.finditer(html):
        if m.start() < pos:
            continue  # inside an element already replaced above
        tag_text = m.group(0)
        if tag_text.rstrip().endswith("/>"):
            continue  # self-closing - data-de/data-en never applies here
        de_m = DATA_DE_RE.search(tag_text)
        en_m = DATA_EN_RE.search(tag_text)
        if not (de_m and en_m):
            continue
        tag = m.group(1)
        content_end, close_end = find_matching_close(html, tag, m.end())
        value = de_m.group(1) if lang == "de" else en_m.group(1)
        cleaned_tag = STRIP_DATA_DE_EN_RE.sub("", tag_text)
        out.append(html[pos:m.start()])
        out.append(cleaned_tag)
        out.append(value)
        pos = content_end
    out.append(html[pos:])
    return "".join(out)


def rewrite_head(html: str, lang: str, body_attrs: dict[str, str], de_url_abs: str, en_url_abs: str) -> str:
    def pick(base: str, fallback_base: str | None = None) -> str:
        key = f"data-{base}-{lang}"
        if key in body_attrs:
            return body_attrs[key]
        if fallback_base:
            return pick(fallback_base)
        raise KeyError(f"{key} missing on <body> and no fallback given")

    title = pick("title")
    description = pick("description")
    og_title = pick("og-title", fallback_base="title")
    og_description = pick("og-description", fallback_base="description")
    own_url_abs = de_url_abs if lang == "de" else en_url_abs

    html = re.sub(r'<html lang="[a-z-]+">', lambda m: f'<html lang="{lang}">', html, count=1)
    html = re.sub(r'<title>.*?</title>', lambda m: f'<title>{title}</title>', html, count=1, flags=re.DOTALL)
    html = re.sub(
        r'(<meta name="description" content=")[^"]*(")',
        lambda m: f'{m.group(1)}{description}{m.group(2)}', html, count=1,
    )
    html = re.sub(
        r'(<meta property="og:title" content=")[^"]*(")',
        lambda m: f'{m.group(1)}{og_title}{m.group(2)}', html, count=1,
    )
    html = re.sub(
        r'(<meta property="og:description" content=")[^"]*(")',
        lambda m: f'{m.group(1)}{og_description}{m.group(2)}', html, count=1,
    )
    html = re.sub(
        r'(<meta property="og:url" content=")[^"]*(")',
        lambda m: f'{m.group(1)}{own_url_abs}{m.group(2)}', html, count=1,
    )
    html = re.sub(
        r'(<link rel="canonical" href=")[^"]*(")',
        lambda m: f'{m.group(1)}{own_url_abs}{m.group(2)}', html, count=1,
    )
    hreflang_block = (
        f'<link rel="alternate" hreflang="de" href="{de_url_abs}">\n'
        f'  <link rel="alternate" hreflang="en" href="{en_url_abs}">\n'
        f'  <link rel="alternate" hreflang="x-default" href="{de_url_abs}">'
    )
    html = re.sub(
        r'(<link rel="canonical" href="[^"]*">)',
        lambda m: f'{m.group(1)}\n  {hreflang_block}', html, count=1,
    )
    return html


def rewrite_assets(html: str, lang: str) -> str:
    docs_base = "/docs/en/" if lang == "en" else "/docs/"
    html = DOCS_HREF_RE.sub(lambda m: f'{m.group(1)}="{docs_base}{m.group(2)}"', html)
    html = ASSET_HREF_RE.sub(lambda m: f'{m.group(1)}="/{m.group(2)}"', html)
    html = ASSET_DIR_HREF_RE.sub(lambda m: f'{m.group(1)}="/{m.group(2)}"', html)
    return html


def build_lang_switch(de_site_url: str, en_site_url: str, lang: str) -> str:
    de_class = "lang-button active" if lang == "de" else "lang-button"
    en_class = "lang-button active" if lang == "en" else "lang-button"
    de_current = ' aria-current="true"' if lang == "de" else ""
    en_current = ' aria-current="true"' if lang == "en" else ""
    return (
        '<div class="language-switch nav-language" aria-label="Sprache">'
        f'<a class="{de_class}" href="{de_site_url}"{de_current}>DE</a>'
        f'<a class="{en_class}" href="{en_site_url}"{en_current}>EN</a>'
        '</div>'
    )


def strip_body_data_attrs(html: str) -> str:
    body_start = html.index("<body")
    m = OPEN_TAG_RE.match(html, body_start)
    if not m:
        raise ValueError("could not parse <body> opening tag for cleanup")
    cleaned = BODY_DATA_ATTR_RE.sub("", m.group(0))
    return html[:m.start()] + cleaned + html[m.end():]


def process_page(page: str, en_dir: Path) -> None:
    src_path = SITE_DIR / page
    original = src_path.read_text(encoding="utf-8")

    canonical_m = re.search(r'<link rel="canonical" href="([^"]*)">', original)
    if not canonical_m:
        sys.exit(f"{page}: no canonical link found")
    canonical = canonical_m.group(1)
    if not canonical.startswith(BASE_URL):
        sys.exit(f"{page}: canonical {canonical!r} doesn't start with {BASE_URL}")
    path = canonical[len(BASE_URL):].lstrip("/")

    de_url_abs = f"{BASE_URL}/{path}"
    en_url_abs = f"{BASE_URL}/en/{path}"
    de_site_url = f"/{path}"
    en_site_url = f"/en/{path}"

    body_start = original.index("<body")
    body_tag_m = OPEN_TAG_RE.match(original, body_start)
    if not body_tag_m:
        sys.exit(f"{page}: could not parse <body> opening tag")
    body_attrs = dict(re.findall(r'(data-[a-z-]+)="([^"]*)"', body_tag_m.group(0)))

    for lang, out_path in (("de", src_path), ("en", en_dir / page)):
        html = bake_language(original, lang)
        html = rewrite_head(html, lang, body_attrs, de_url_abs, en_url_abs)
        html = rewrite_assets(html, lang)
        lang_switch = build_lang_switch(de_site_url, en_site_url, lang)
        html, count = LANG_SWITCH_RE.subn(lambda m: lang_switch, html, count=1)
        if count != 1:
            sys.exit(f"{page}: language-switch block not found")
        html = strip_body_data_attrs(html)
        out_path.write_text(html, encoding="utf-8")
        print(f"wrote {out_path.relative_to(SITE_DIR.parent)}")


def main() -> None:
    en_dir = SITE_DIR / "en"
    en_dir.mkdir(exist_ok=True)
    for page in PAGES:
        process_page(page, en_dir)


if __name__ == "__main__":
    main()
