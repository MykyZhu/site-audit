package logger

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"site-audit/audit"
	"sort"
	"strconv"
	"strings"
)

// Write writes .log, .txt, and .csv files into the given directory
func Write(results []audit.PageResult, dir string, baseName string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	content := BuildContent(results)

	logPath := filepath.Join(dir, baseName+".log")
	txtPath := filepath.Join(dir, baseName+".txt")
	csvPath := filepath.Join(dir, baseName+".csv")

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	if err := os.WriteFile(txtPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("txt: %w", err)
	}
	if err := writeCSV(results, csvPath); err != nil {
		return fmt.Errorf("csv: %w", err)
	}
	return nil
}

// buildContent builds the full report string shared by .log and .txt
// BuildContent builds the full report string (exported for TUI use)
func BuildContent(results []audit.PageResult) string {
	var sb strings.Builder

	sb.WriteString("SITE AUDIT REPORT\n")
	if len(results) > 0 {
		sb.WriteString(fmt.Sprintf("Date: %s\n", results[0].Date.Format("2006-01-02 15:04:05")))
	}
	sb.WriteString("\n")

	writeSummary(results, &sb)

	sb.WriteString("DETAIL\n")
	sb.WriteString(strings.Repeat("=", 80) + "\n\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("PAGE:       %s\n", r.URL))
		if r.Title != "" {
			sb.WriteString(fmt.Sprintf("TITLE:      %s\n", r.Title))
		}
		sb.WriteString(fmt.Sprintf("LOAD TIME:  %dms\n", r.TotalTimeMs))

		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("ERROR:      %s\n", r.Error))
			sb.WriteString("\n")
			continue
		}

		var totalBytes int64
		for _, res := range r.Resources {
			totalBytes += res.TransferB
		}
		sb.WriteString(fmt.Sprintf("RESOURCES:  %d loaded, %s total\n",
			len(r.Resources), formatBytes(totalBytes)))
		sb.WriteString("\n")

		sb.WriteString(fmt.Sprintf("  %-10s %-10s %-12s %-10s %s\n",
			"TYPE", "DURATION", "SIZE", "START", "URL"))
		sb.WriteString("  " + strings.Repeat("-", 76) + "\n")

		// Sort largest resources first
		sorted := make([]audit.Resource, len(r.Resources))
		copy(sorted, r.Resources)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].TransferB > sorted[j].TransferB
		})

		for _, res := range sorted {
			sb.WriteString(fmt.Sprintf("  %-10s %-10s %-12s %-10s %s\n",
				truncate(res.Type, 9),
				fmt.Sprintf("%.0fms", res.DurationMs),
				formatBytes(res.TransferB),
				fmt.Sprintf("%.0fms", res.StartTimeMs),
				res.URL,
			))
		}
		sb.WriteString("\n" + strings.Repeat("=", 80) + "\n\n")
	}

	return sb.String()
}

// writeSummary builds the ASCII summary table with average and total rows
func writeSummary(results []audit.PageResult, sb *strings.Builder) {
	const colPage = 40
	const colTime = 10
	const colRes = 10
	const colSize = 10
	const colStatus = 8
	tableWidth := colPage + colTime + colRes + colSize + colStatus + 16

	divider := "+" + strings.Repeat("-", colPage+2) +
		"+" + strings.Repeat("-", colTime+2) +
		"+" + strings.Repeat("-", colRes+2) +
		"+" + strings.Repeat("-", colSize+2) +
		"+" + strings.Repeat("-", colStatus+2) + "+\n"

	row := func(page, t, res, size, status string) string {
		return fmt.Sprintf("| %-*s | %-*s | %-*s | %-*s | %-*s |\n",
			colPage, truncate(page, colPage),
			colTime, t,
			colRes, res,
			colSize, size,
			colStatus, status,
		)
	}

	sb.WriteString("SUMMARY\n")
	sb.WriteString(strings.Repeat("=", tableWidth) + "\n")
	sb.WriteString(divider)
	sb.WriteString(row("PAGE", "LOAD TIME", "RESOURCES", "TOTAL SIZE", "STATUS"))
	sb.WriteString(divider)

	var totalTimeMs int64
	var totalResources int
	var totalBytes int64
	errCount := 0
	n := len(results)

	for _, r := range results {
		var pageBytes int64
		for _, res := range r.Resources {
			pageBytes += res.TransferB
		}
		totalBytes += pageBytes
		totalTimeMs += r.TotalTimeMs
		totalResources += len(r.Resources)

		status := "OK"
		if r.Error != "" {
			status = "ERROR"
			errCount++
		}

		sb.WriteString(row(
			r.URL,
			fmt.Sprintf("%dms", r.TotalTimeMs),
			fmt.Sprintf("%d", len(r.Resources)),
			formatBytes(pageBytes),
			status,
		))
	}

	// Average row
	sb.WriteString(divider)
	if n > 0 {
		sb.WriteString(row(
			fmt.Sprintf("AVERAGE (%d pages)", n),
			fmt.Sprintf("%dms", totalTimeMs/int64(n)),
			fmt.Sprintf("%d", totalResources/n),
			formatBytes(totalBytes/int64(n)),
			"",
		))
	}

	// Total row
	totalStatus := fmt.Sprintf("%d err", errCount)
	if errCount == 0 {
		totalStatus = "all ok"
	}
	sb.WriteString(row(
		fmt.Sprintf("TOTAL (%d pages)", n),
		fmt.Sprintf("%dms", totalTimeMs),
		fmt.Sprintf("%d", totalResources),
		formatBytes(totalBytes),
		totalStatus,
	))
	sb.WriteString(divider)
	sb.WriteString("\n\n")
}

func writeCSV(results []audit.PageResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{
		"page_url", "page_title", "page_load_ms", "page_error",
		"resource_type", "resource_duration_ms", "resource_bytes",
		"resource_start_ms", "resource_url",
	})

	for _, r := range results {
		if r.Error != "" {
			w.Write([]string{r.URL, r.Title,
				strconv.FormatInt(r.TotalTimeMs, 10), r.Error,
				"", "", "", "", "",
			})
			continue
		}
		for _, res := range r.Resources {
			w.Write([]string{
				r.URL,
				r.Title,
				strconv.FormatInt(r.TotalTimeMs, 10),
				"",
				res.Type,
				strconv.FormatFloat(res.DurationMs, 'f', 1, 64),
				strconv.FormatInt(res.TransferB, 10),
				strconv.FormatFloat(res.StartTimeMs, 'f', 1, 64),
				res.URL,
			})
		}
	}

	w.Flush()
	return w.Error()
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
