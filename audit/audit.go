package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Resource is a single loaded asset on the page
type Resource struct {
	URL         string  `json:"url"`
	Type        string  `json:"type"`
	MimeType    string  `json:"mime_type"`
	DurationMs  float64 `json:"duration_ms"`
	TransferB   int64   `json:"transfer_bytes"`
	StartTimeMs float64 `json:"start_time_ms"`
	Status      int     `json:"status"`
}

// PageResult holds everything collected for one audited page
type PageResult struct {
	URL         string
	Title       string
	Date        time.Time
	TotalTimeMs int64
	Resources   []Resource
	Error       string
}

type inFlight struct {
	url       string
	startTime time.Time
	reqType   string
	mimeType  string
	status    int
}

func findChrome() (string, error) {
	if env := os.Getenv("CHROME_PATH"); env != "" {
		return env, nil
	}
	home, _ := os.UserHomeDir()

	shellDir := filepath.Join(home, "chrome-headless-shell")
	if entries, err := os.ReadDir(shellDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			versionDir := filepath.Join(shellDir, e.Name())
			platforms, _ := os.ReadDir(versionDir)
			for _, p := range platforms {
				candidate := filepath.Join(versionDir, p.Name(), "chrome-headless-shell")
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
		}
	}

	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	pathDirs := strings.Split(os.Getenv("PATH"), ":")
	names := []string{"chrome-headless-shell", "google-chrome", "chromium", "chromium-browser"}
	for _, dir := range pathDirs {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf(
		"could not find Chrome or chrome-headless-shell.\n" +
			"Install with: npx @puppeteer/browsers install chrome-headless-shell\n" +
			"Or set the CHROME_PATH environment variable to the binary path.",
	)
}

// Run audits a single URL via CDP Network events — captures every request
// the browser makes regardless of origin, including cross-origin CDNs.
func Run(pageURL string) PageResult {
	result := PageResult{
		URL:  pageURL,
		Date: time.Now(),
	}

	chromePath, err := findChrome()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		// Disable cache so we always get real transfer sizes
		chromedp.Flag("disable-application-cache", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	ctx, cancelTimeout := context.WithTimeout(ctx, 90*time.Second)
	defer cancelTimeout()

	var mu sync.Mutex
	requests := map[network.RequestID]*inFlight{}
	var completed []Resource
	pageStart := time.Now()

	// CDP network events fire at the browser engine level — no CORS filtering,
	// captures everything: own server, Storyblok, CDNs, analytics, fonts, etc.
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		mu.Lock()
		defer mu.Unlock()

		switch e := ev.(type) {

		case *network.EventRequestWillBeSent:
			// Skip data URIs — they're inline, not real network requests
			if strings.HasPrefix(e.Request.URL, "data:") {
				return
			}
			requests[e.RequestID] = &inFlight{
				url:       e.Request.URL,
				startTime: time.Now(),
				reqType:   strings.ToLower(e.Type.String()),
			}

		case *network.EventResponseReceived:
			if req, ok := requests[e.RequestID]; ok {
				req.mimeType = e.Response.MimeType
				req.status = int(e.Response.Status)
			}

		case *network.EventLoadingFinished:
			req, ok := requests[e.RequestID]
			if !ok {
				return
			}
			durationMs := float64(time.Since(req.startTime).Milliseconds())
			startMs := float64(req.startTime.Sub(pageStart).Milliseconds())
			if startMs < 0 {
				startMs = 0
			}
			completed = append(completed, Resource{
				URL:         req.url,
				Type:        classifyType(req.reqType, req.mimeType),
				MimeType:    req.mimeType,
				DurationMs:  durationMs,
				TransferB:   int64(e.EncodedDataLength),
				StartTimeMs: startMs,
				Status:      req.status,
			})
			delete(requests, e.RequestID)

		case *network.EventLoadingFailed:
			delete(requests, e.RequestID)
		}
	})

	var title string

	err = chromedp.Run(ctx,
		// Enable network capture with cache disabled so sizes are real
		network.Enable(),
		network.SetCacheDisabled(true),
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// Wait for lazy-loaded assets, JS bundles, and async fetches to settle
		chromedp.Sleep(3*time.Second),
		chromedp.Title(&title),
	)

	result.TotalTimeMs = time.Since(pageStart).Milliseconds()
	result.Title = title

	if err != nil {
		result.Error = fmt.Sprintf("browser error: %v", err)
		return result
	}

	mu.Lock()
	result.Resources = completed
	mu.Unlock()

	return result
}

// classifyType gives a clean human-readable type label
func classifyType(cdpType, mimeType string) string {
	// CDP type is usually good enough
	switch cdpType {
	case "document":
		return "document"
	case "stylesheet":
		return "css"
	case "script":
		return "script"
	case "image":
		return "image"
	case "media":
		return "media"
	case "font":
		return "font"
	case "fetch", "xhr":
		return cdpType
	}
	// Fall back to MIME type for anything else
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/"):
		return "media"
	case strings.Contains(mimeType, "javascript"):
		return "script"
	case strings.Contains(mimeType, "css"):
		return "css"
	case strings.HasPrefix(mimeType, "font/") || strings.Contains(mimeType, "woff"):
		return "font"
	case strings.Contains(mimeType, "json"):
		return "fetch"
	default:
		if cdpType != "" {
			return cdpType
		}
		return "other"
	}
}
