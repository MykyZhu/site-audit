package logger

import (
	"site-audit/audit"
	"strings"
	"testing"
	"time"
)

// Each column must be coloured as a whole, including its trailing separator.
func TestColorResourceRowColumns(t *testing.T) {
	results := []audit.PageResult{{
		URL: "https://x.com/", Date: time.Now(), TotalTimeMs: 100,
		Resources: []audit.Resource{{URL: "https://x.com/app.js", Type: "script", DurationMs: 320, TransferB: 2048, StartTimeMs: 12}},
	}}
	var row string
	for _, l := range strings.Split(BuildContent(results), "\n") {
		if strings.HasPrefix(l, "  script") {
			row = l
		}
	}
	if row == "" {
		t.Fatal("resource row not found")
	}
	rest := row[2:]
	fields := []string{rest[:11], rest[11:22], rest[22:35], rest[35:46], rest[46:]}
	if strings.TrimSpace(fields[0]) != "script" || strings.TrimSpace(fields[1]) != "320ms" {
		t.Errorf("unexpected split: %q", fields)
	}
	if strings.TrimSpace(fields[2]) != "2.0KB" || strings.TrimSpace(fields[3]) != "12ms" || fields[4] != "https://x.com/app.js" {
		t.Errorf("unexpected split: %q", fields)
	}
}

func TestTruncateUTF8(t *testing.T) {
	if got := truncate("héllo", 2); got != "hé" {
		t.Errorf("got %q", got)
	}
}
