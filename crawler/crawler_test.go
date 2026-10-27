package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCleanURL(t *testing.T) {
	cases := map[string]string{
		"https://site.com":          "https://site.com/",
		"https://site.com/":         "https://site.com/",
		"https://site.com/about/":   "https://site.com/about",
		"https://site.com/a#frag":   "https://site.com/a",
		"https://site.com/a/?q=1":   "https://site.com/a?q=1",
		"https://site.com/?q=1#top": "https://site.com/?q=1",
	}
	for in, want := range cases {
		if got := cleanURL(in); got != want {
			t.Errorf("cleanURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCrawlFollowsRedirectAndDedupes(t *testing.T) {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home" {
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}
		fmt.Fprintf(w, `<html><head><title>Home</title></head><body>
			<a href="/home">self</a>
			<a href="/about/">about</a>
			<a href="/about">about again</a>
			<a href="%s/contact#x">abs</a>
			<a href="/file.pdf?dl=1">pdf</a>
			<a href="mailto:a@b.c">mail</a>
			<a href="https://other.example/">ext</a>
		</body></html>`, srv.URL)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	pages, err := Crawl(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range pages {
		got = append(got, p.URL)
	}
	want := []string{srv.URL + "/home", srv.URL + "/about", srv.URL + "/contact"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if pages[0].Title != "Home" {
		t.Errorf("title = %q", pages[0].Title)
	}
}
