# site-audit

A CLI tool that crawls a website, lets you pick which pages to audit, then
logs load time and resource data (images, scripts, CSS, video etc.) for each
selected page using a real headless Chromium browser.

---

## Requirements

- Go 1.22+  →  https://go.dev/dl/
- Google Chrome installed (the tool drives your existing Chrome, no extra driver needed)

---

## Setup

```bash
# 1. Clone or copy this folder, then enter it
cd site-audit

# 2. Download dependencies
go mod tidy

# 3. Build the binary
go build -o site-audit .
```

---

## Usage

```bash
# Basic — prompts for URL, writes audit.log (plain text)
./site-audit

# Pass URL directly
./site-audit https://yoursite.com

# Choose output file and format
./site-audit https://yoursite.com -out results.csv -format csv
./site-audit https://yoursite.com -out results.json -format json
```

### Flags

| Flag      | Default     | Description                        |
|-----------|-------------|------------------------------------|
| `-out`    | `audit.log` | Output file path                   |
| `-format` | `txt`       | Output format: `txt`, `csv`, `json`|

---

## How it works

1. **Crawl** — fetches your entry URL, parses all `<a href>` links, keeps only
   internal ones (same domain, not files like .pdf/.zip/images).

2. **Select** — shows an interactive list in the terminal. Navigate with arrow
   keys, toggle pages with `space`, confirm with `enter`.

   ```
   ◉  https://yoursite.com            (Home)
   ○  https://yoursite.com/about      (About Us)
   ◉  https://yoursite.com/work       (Our Work)
   ○  https://yoursite.com/contact    (Contact)

   space: toggle  •  a: all  •  n: none  •  enter: confirm  •  q: quit
   ```

3. **Audit** — opens each selected page in a headless Chrome, waits for it to
   settle, then reads the browser's Performance Resource Timing API to get
   accurate per-asset data.

4. **Log** — writes a file with load time, resource count, and a row per asset
   showing type, duration, size, and start time.

---

## Example txt output

```
SITE AUDIT REPORT
================================================================================

PAGE:       https://yoursite.com
TITLE:      Home — Your Site
DATE:       2026-06-24 14:00:00
LOAD TIME:  3240ms
RESOURCES:  47 loaded, 4.2MB total

  TYPE       DURATION   SIZE         START      URL
  ----------------------------------------------------------------------------
  script     320ms      82.3KB       120ms      https://yoursite.com/app.js
  img        210ms      200.0KB      340ms      https://yoursite.com/hero.jpg
  css        88ms       12.1KB       118ms      https://yoursite.com/style.css
  video      940ms      2.0MB        500ms      https://yoursite.com/bg.mp4
```
