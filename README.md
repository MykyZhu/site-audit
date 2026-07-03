# site-audit

A terminal UI app for auditing website performance. Crawls a site, lets you pick which pages to scan, then captures every network resource loaded on each page — including assets from external CDNs — and logs timing, size, and type data.

![Platform: macOS](https://img.shields.io/badge/platform-macOS-lightgrey)

---

## Features

- **Full-screen TUI** — keyboard-driven interface with panels for URL input, saved URLs, history, and a log viewer
- **Shallow crawler** — finds all internal pages linked from your entry URL and lets you choose which ones to audit
- **Complete resource capture** — uses Chrome DevTools Protocol (CDP) to intercept every network request at the browser engine level, including assets served from external CDNs (Storyblok, Cloudfront, Google Fonts, etc.) that other tools miss
- **Syntax-highlighted log viewer** — colour-coded results right inside the terminal after each scan
- **History pane** — every scan is saved to history; select an entry to re-open its log
- **Saved URLs pane** — pin frequently tested URLs for quick re-scanning
- **Persistent logs** — each scan writes `.log`, `.txt`, and `.csv` files to `~/Desktop/_Logs/`
- **Open log files directly** — pass a `.log` file as an argument to open it straight in the viewer
- **History sync** — on launch the app checks the `_Logs` folder and reconciles history automatically (handles deleted or manually added files)

---

## Platform support

| Platform | Supported |
|---|---|
| macOS — Apple Silicon (M1/M2/M3) | ✅ |
| macOS — Intel | ✅ |
| Linux | ⚠️ Builds fine but untested |
| Windows | ❌ Not supported |

The app depends on `chrome-headless-shell` for page scanning, which is available on macOS and Linux but not Windows.

---

## Installation

### via Homebrew (recommended)

```bash
brew tap MykyZhu/site-audit
brew install site-audit
```

Then install the headless Chrome runtime (one-time, required for scanning):

```bash
npx @puppeteer/browsers install chrome-headless-shell
```

### Build from source

Requires Go 1.22+ and Node.js (for the Chrome install step).

```bash
git clone https://github.com/MykyZhu/site-audit
cd site-audit
go mod tidy
go build -o site-audit .

# Install chrome-headless-shell (required for scanning)
npx @puppeteer/browsers install chrome-headless-shell

# Optionally make it available system-wide
ln -sf "$(pwd)/site-audit" ~/bin/site-audit
```

---

## Usage

```bash
# Launch the TUI
site-audit

# Open a specific log file directly in the viewer
site-audit ~/Desktop/_Logs/mysite.com_2026-07-01_14-00-00.log

# Check version
site-audit --version
```

---

## TUI layout

```
╭─ URL ──────────────────╮ ╭─────────────────────────────────────────────────╮
│ > https://...          │ │                                                   │
╰────────────────────────╯ │   Right panel:                                    │
╭─ SAVED ────────────────╮ │   • Welcome screen                               │
│   mysite.com           │ │   • Page selector (after crawl)                  │
│   other.com            │ │   • Live audit progress                           │
╰────────────────────────╯ │   • Syntax-highlighted log viewer                │
╭─ HISTORY ──────────────╮ │                                                   │
│   mysite.com  07-01    │ │                                                   │
│   other.com   06-29    │ │                                                   │
╰────────────────────────╯ │                                                   │
╭─ Open logs in Finder ──╮ │                                                   │
╰────────────────────────╯ ╰───────────────────────────────────────────────────╯
```

---

## Keyboard reference

### Always

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Move focus between panels |
| `ctrl+c` | Quit |

### URL panel (focused)

| Key | Action |
|---|---|
| Type | Enter a URL to scan |
| `enter` | Start crawl |
| `↓` | Move focus to Saved panel |

### Saved panel

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate entries |
| `enter` / `space` | Run a new scan for this URL |
| `d` | Delete entry |

### History panel

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate entries |
| `enter` / `space` | Open this scan's log in the viewer |
| `s` | Save this URL to the Saved panel |
| `d` | Delete entry from history |

### Page selector (right panel, after crawl)

| Key | Action |
|---|---|
| `↑` / `↓` or `j` / `k` | Navigate pages |
| `space` | Toggle page selection |
| `a` | Select all |
| `n` | Deselect all |
| `enter` | Start auditing selected pages |
| `esc` | Go back |

### Log viewer (right panel, after audit or from history)

| Key | Action |
|---|---|
| `↑` / `↓` / `pgup` / `pgdn` | Scroll |
| Touchpad / mouse wheel | Scroll (supported in Ghostty and most modern terminals) |
| `q` / `esc` | Back to main screen |

---

## Log output

Every scan saves three files to `~/Desktop/_Logs/`, named after the site and timestamp:

| File | Contents |
|---|---|
| `mysite.com_2026-07-01_14-00-00.log` | Full human-readable report |
| `mysite.com_2026-07-01_14-00-00.txt` | Same content, `.txt` extension |
| `mysite.com_2026-07-01_14-00-00.csv` | Machine-readable, one row per resource |

Each log starts with a summary table showing load time, resource count, and total transfer size per page, with average and total rows. The detail section below lists every resource sorted by size (largest first), showing type, load duration, size, start time, and URL.

---

## How resource capture works

Most browser-based tools read resource timing data from the JavaScript Performance API. That API is subject to CORS — cross-origin servers need to explicitly opt in with a `Timing-Allow-Origin` header before they'll share size and timing data. Most CDNs don't set this header, so assets served from them appear with zero size.

`site-audit` instead attaches to Chrome at the DevTools Protocol level and listens to raw network events (`Network.requestWillBeSent`, `Network.loadingFinished`). These fire before any CORS logic applies and give accurate sizes and timings for everything the browser loads — your own server, Storyblok, Cloudfront, Google Fonts, analytics scripts, video CDNs, all of it.

Browser cache is also disabled during scans so transfer sizes always reflect what a first-time visitor would download.

---

## Configuration

No config file needed. The app stores two small JSON files in `~/.config/site-audit/`:

- `history.json` — scan history (auto-managed)
- `saved.json` — your pinned URLs

To point the app at a specific Chrome binary (e.g. if auto-detection fails):

```bash
export CHROME_PATH=/path/to/chrome-headless-shell
site-audit
```

---


