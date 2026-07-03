package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Entry struct {
	SiteName string    `json:"site_name"`
	LogPath  string    `json:"log_path"`
	Date     time.Time `json:"date"`
}

type SavedEntry struct {
	SiteName string    `json:"site_name"`
	URL      string    `json:"url"`
	Date     time.Time `json:"date"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "site-audit")
}

func LogsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop", "_Logs")
}

func historyPath() string { return filepath.Join(configDir(), "history.json") }
func savedPath() string   { return filepath.Join(configDir(), "saved.json") }

// ── History ───────────────────────────────────────────────────────────────────

func Load() []Entry {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return nil
	}
	var entries []Entry
	json.Unmarshal(data, &entries)
	return entries
}

// Sync reconciles history with what's actually on disk in the _Logs folder:
//   - removes entries whose log files no longer exist
//   - adds any .log files in the folder not already tracked
func Sync() {
	entries := Load()

	// Remove entries with missing files
	valid := entries[:0]
	for _, e := range entries {
		if _, err := os.Stat(e.LogPath); err == nil {
			valid = append(valid, e)
		}
	}

	// Index existing paths so we don't duplicate
	known := map[string]bool{}
	for _, e := range valid {
		known[e.LogPath] = true
	}

	// Scan _Logs folder for any untracked .log files
	logsDir := LogsDir()
	dirEntries, err := os.ReadDir(logsDir)
	if err == nil {
		for _, de := range dirEntries {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".log") {
				continue
			}
			fullPath := filepath.Join(logsDir, de.Name())
			if known[fullPath] {
				continue
			}
			// Derive site name from filename: "mysite.com_2026-07-01_12-00-00.log"
			name := strings.TrimSuffix(de.Name(), ".log")
			// Everything before the first _YYYY- part is the site name
			siteName := name
			if idx := strings.Index(name, "_20"); idx != -1 {
				siteName = name[:idx]
			}
			info, _ := de.Info()
			modTime := time.Now()
			if info != nil {
				modTime = info.ModTime()
			}
			valid = append(valid, Entry{
				SiteName: siteName,
				LogPath:  fullPath,
				Date:     modTime,
			})
		}
	}

	// Sort newest first
	sort.Slice(valid, func(i, j int) bool {
		return valid[i].Date.After(valid[j].Date)
	})

	if len(valid) > 50 {
		valid = valid[:50]
	}

	save(historyPath(), valid)
}

func Add(siteName, logPath string) {
	entries := Load()
	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.LogPath != logPath {
			filtered = append(filtered, e)
		}
	}
	entry := Entry{SiteName: siteName, LogPath: logPath, Date: time.Now()}
	filtered = append([]Entry{entry}, filtered...)
	if len(filtered) > 50 {
		filtered = filtered[:50]
	}
	save(historyPath(), filtered)
}

func Remove(index int) {
	entries := Load()
	if index < 0 || index >= len(entries) {
		return
	}
	entries = append(entries[:index], entries[index+1:]...)
	save(historyPath(), entries)
}

// ── Saved ─────────────────────────────────────────────────────────────────────

func LoadSaved() []SavedEntry {
	data, err := os.ReadFile(savedPath())
	if err != nil {
		return nil
	}
	var entries []SavedEntry
	json.Unmarshal(data, &entries)
	return entries
}

func AddSaved(siteName, url string) {
	entries := LoadSaved()
	for _, e := range entries {
		if e.URL == url {
			return
		}
	}
	entry := SavedEntry{SiteName: siteName, URL: url, Date: time.Now()}
	entries = append([]SavedEntry{entry}, entries...)
	save(savedPath(), entries)
}

func RemoveSaved(index int) {
	entries := LoadSaved()
	if index < 0 || index >= len(entries) {
		return
	}
	entries = append(entries[:index], entries[index+1:]...)
	save(savedPath(), entries)
}

// ── Internal ──────────────────────────────────────────────────────────────────

func save(path string, v interface{}) {
	os.MkdirAll(configDir(), 0755)
	data, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(path, data, 0644)
}
