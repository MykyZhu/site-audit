package tui

import (
	"site-audit/audit"
	"site-audit/crawler"
	"testing"
)

func TestListWindow(t *testing.T) {
	cases := []struct{ cur, n, h, s, e int }{
		{0, 3, 10, 0, 3},
		{0, 100, 10, 0, 10},
		{15, 100, 10, 6, 16},
		{99, 100, 10, 90, 100},
	}
	for _, c := range cases {
		s, e := listWindow(c.cur, c.n, c.h)
		if s != c.s || e != c.e {
			t.Errorf("listWindow(%d,%d,%d) = %d,%d want %d,%d", c.cur, c.n, c.h, s, e, c.s, c.e)
		}
	}
}

// A result from an older run must not touch the current run (it used to panic
// with index out of range when the new run had fewer pages).
func TestStaleAuditResultIgnored(t *testing.T) {
	m := Model{runID: 2, state: stateAuditing, auditLines: []string{"pending"}, auditTotal: 1}
	out, _ := m.Update(auditPageDoneMsg{run: 1, index: 5, result: audit.PageResult{URL: "x"}, pages: make([]crawler.Page, 6)})
	if got := out.(Model); len(got.auditResults) != 0 || got.auditLines[0] != "pending" {
		t.Fatal("stale result modified state")
	}
}

func TestStartScanBlockedWhileBusy(t *testing.T) {
	m := Model{runID: 3, state: stateAuditing}
	if cmd := m.startScan("https://a.com"); cmd != nil || m.runID != 3 {
		t.Fatal("scan started while busy")
	}
}

func TestSiteBaseName(t *testing.T) {
	if got := siteBaseName("https://www.my-site.com/path?x"); got != "my-site.com" {
		t.Errorf("got %q", got)
	}
}
