package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Page represents a discovered internal page
type Page struct {
	URL   string
	Title string
}

// Crawl fetches the entry URL and returns all unique internal pages found on it (shallow)
func Crawl(entryURL string) ([]Page, error) {
	base, err := url.Parse(entryURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Normalize — always use scheme + host as the boundary for "internal"
	base.Fragment = ""
	base.RawQuery = ""

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Get(entryURL)
	if err != nil {
		return nil, fmt.Errorf("could not fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not parse HTML: %w", err)
	}

	seen := map[string]bool{}
	var pages []Page

	// Always include the entry page itself
	entryTitle := extractTitle(doc)
	cleanEntry := cleanURL(entryURL)
	seen[cleanEntry] = true
	pages = append(pages, Page{URL: cleanEntry, Title: entryTitle})

	// Walk the DOM looking for <a href=...>
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := strings.TrimSpace(attr.Val)
					resolved := resolveLink(base, link)
					if resolved == "" {
						break
					}
					if !seen[resolved] && isInternal(base, resolved) {
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

	// Skip file extensions that aren't pages
	lower := strings.ToLower(href)
	skipExts := []string{".pdf", ".zip", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".mp4", ".webp", ".ico"}
	for _, ext := range skipExts {
		if strings.HasSuffix(lower, ext) {
			return ""
		}
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return ""
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

// cleanURL strips fragments and trailing slashes for deduplication
func cleanURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	result := u.String()
	if result != u.Scheme+"://"+u.Host+"/" {
		result = strings.TrimRight(result, "/")
	}
	return result
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
