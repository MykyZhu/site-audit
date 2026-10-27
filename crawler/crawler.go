package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 site-audit"

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Files that aren't pages
var skipExts = []string{".pdf", ".zip", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".mp4", ".webp", ".ico"}

// Page represents a discovered internal page
type Page struct {
	URL   string
	Title string
}

// Crawl fetches the entry URL and returns all unique internal pages found on it (shallow)
func Crawl(entryURL string) ([]Page, error) {
	if _, err := url.Parse(entryURL); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, entryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	// Some sites reject Go's default User-Agent
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	// Use the final URL after redirects (e.g. example.com → www.example.com)
	// as the base, otherwise every link looks external.
	finalURL := resp.Request.URL.String()
	base := *resp.Request.URL
	base.Fragment = ""
	base.RawQuery = ""

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not parse HTML: %w", err)
	}

	seen := map[string]bool{}
	var pages []Page

	// Always include the entry page itself
	entryTitle := extractTitle(doc)
	cleanEntry := cleanURL(finalURL)
	seen[cleanEntry] = true
	pages = append(pages, Page{URL: cleanEntry, Title: entryTitle})

	// Walk the DOM looking for <a href=...>
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := strings.TrimSpace(attr.Val)
					resolved := resolveLink(&base, link)
					if resolved == "" {
						break
					}
					if !seen[resolved] && isInternal(&base, resolved) {
						seen[resolved] = true
						pages = append(pages, Page{URL: resolved, Title: resolved})
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return pages, nil
}

// resolveLink turns a raw href into an absolute URL string, or returns ""
func resolveLink(base *url.URL, href string) string {
	// Skip non-page links
	if href == "" ||
		strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "javascript:") {
		return ""
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}

	// Skip file extensions that aren't pages (ignoring any ?query)
	lower := strings.ToLower(parsed.Path)
	for _, ext := range skipExts {
		if strings.HasSuffix(lower, ext) {
			return ""
		}
	}

	resolved := base.ResolveReference(parsed)
	return cleanURL(resolved.String())
}

// isInternal checks the resolved URL shares the same host as the base
func isInternal(base *url.URL, resolved string) bool {
	u, err := url.Parse(resolved)
	if err != nil {
		return false
	}
	return u.Host == base.Host
}

// cleanURL strips fragments and trailing slashes for deduplication.
// "https://site.com" and "https://site.com/" both become "https://site.com/".
func cleanURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.RawFragment = ""
	if u.Path == "" {
		u.Path = "/"
	} else if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
		u.RawPath = ""
		if u.Path == "" {
			u.Path = "/"
		}
	}
	return u.String()
}

// extractTitle pulls the <title> text from a parsed doc
func extractTitle(doc *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = strings.TrimSpace(n.FirstChild.Data)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title
}
