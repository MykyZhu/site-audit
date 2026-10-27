//go:build live

package audit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLiveSharedBrowser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow.js":
			time.Sleep(300 * time.Millisecond)
			w.Header().Set("Content-Type", "application/javascript")
			fmt.Fprint(w, "console.log(1)")
		default:
			fmt.Fprint(w, `<html><head><title>T</title><script src="/slow.js"></script></head><body>hi</body></html>`)
		}
	}))
	defer srv.Close()

	t0 := time.Now()
	b, err := NewBrowser()
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	t.Logf("browser start: %v", time.Since(t0))
	for i := 0; i < 3; i++ {
		t1 := time.Now()
		r := b.Run(srv.URL + fmt.Sprintf("/p%d", i))
		t.Logf("page %d: wall=%v load=%dms err=%q title=%q", i, time.Since(t1), r.TotalTimeMs, r.Error, r.Title)
		for _, res := range r.Resources {
			t.Logf("   %-8s %5.0fms start=%4.0fms %dB %s", res.Type, res.DurationMs, res.StartTimeMs, res.TransferB, res.URL)
		}
		if r.Error != "" || len(r.Resources) != 2 {
			t.Fatalf("bad result %+v", r)
		}
	}
}
