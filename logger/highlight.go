package logger

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colour palette for the TUI viewer
var (
	hAccent  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))   // blue — headers
	hMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))                // grey — dividers
	hGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))               // green — ok/sizes
	hRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))                // red — errors
	hYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))               // yellow — timing
	hCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))               // cyan — URLs
	hBold    = lipgloss.NewStyle().Bold(true)                                     // bold — labels
	hDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))                // normal text

	// Resource type colours
	typeColors = map[string]lipgloss.Style{
		"script":   lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // yellow
		"css":      lipgloss.NewStyle().Foreground(lipgloss.Color("13")), // magenta
		"image":    lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // green
		"media":    lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // green
		"font":     lipgloss.NewStyle().Foreground(lipgloss.Color("12")), // blue
		"fetch":    lipgloss.NewStyle().Foreground(lipgloss.Color("14")), // cyan
		"xhr":      lipgloss.NewStyle().Foreground(lipgloss.Color("14")), // cyan
		"document": lipgloss.NewStyle().Foreground(lipgloss.Color("7")),  // white
		"other":    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),  // grey
	}
)

// Highlight takes a plain-text log string and returns an ANSI-coloured version
// for display in the TUI viewport. The saved .log/.txt files are never touched.
func Highlight(plain string) string {
	lines := strings.Split(plain, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, highlightLine(line))
	}
	return strings.Join(out, "\n")
}

func highlightLine(line string) string {
	// ── Blank line
	if strings.TrimSpace(line) == "" {
		return line
	}

	trimmed := strings.TrimSpace(line)

	// ── Top-level section headers: "SITE AUDIT REPORT", "SUMMARY", "DETAIL"
	switch trimmed {
	case "SITE AUDIT REPORT", "SUMMARY", "DETAIL":
		return hAccent.Render(line)
	}

	// ── Date line
	if strings.HasPrefix(trimmed, "Date:") {
		return hMuted.Render(line)
	}

	// ── Table dividers (lines of = or - or +---+)
	if isRepeat(trimmed, '=') || isRepeat(trimmed, '-') {
		return hMuted.Render(line)
	}
	if strings.HasPrefix(trimmed, "+") && strings.HasSuffix(trimmed, "+") && strings.ContainsRune(trimmed, '-') {
		return hMuted.Render(line)
	}

	// ── Table header row: "| PAGE | LOAD TIME | ..."
	if strings.HasPrefix(trimmed, "|") && strings.Contains(line, "LOAD TIME") {
		return hBold.Render(line)
	}

	// ── Summary table data rows: "| https://... | 3200ms | 47 | 1.4MB | OK |"
	if strings.HasPrefix(trimmed, "|") {
		return colorSummaryRow(line)
	}

	// ── Page-level labels: "PAGE:", "TITLE:", "LOAD TIME:", "RESOURCES:", "ERROR:"
	if strings.HasPrefix(trimmed, "ERROR:") {
		return hRed.Render(line)
	}
	for _, label := range []string{"PAGE:", "TITLE:", "LOAD TIME:", "RESOURCES:"} {
		if strings.HasPrefix(trimmed, label) {
			return colorLabelLine(line, label)
		}
	}

	// ── Resource table column header: "  TYPE  DURATION  SIZE  START  URL"
	if strings.Contains(line, "DURATION") && strings.Contains(line, "SIZE") && strings.Contains(line, "START") {
		return hBold.Render(line)
	}

	// ── Resource table row: "  script    320ms    82.3KB  ..."
	if len(line) > 2 && line[0] == ' ' && line[1] == ' ' && line[2] != ' ' && line[2] != '-' {
		return colorResourceRow(line)
	}

	// ── "AVERAGE" and "TOTAL" rows embedded in table (already caught by | prefix above,
	//    but catch bare versions just in case)
	if strings.HasPrefix(trimmed, "AVERAGE") || strings.HasPrefix(trimmed, "TOTAL") {
		return hBold.Render(line)
	}

	return hDim.Render(line)
}

// colorSummaryRow colours the cells of a summary table row
func colorSummaryRow(line string) string {
	// Split on | keeping the pipes, recolour per cell
	parts := strings.Split(line, "|")
	// parts[0] = "" (before first |), parts[len-1] = "" (after last |)
	// inner parts are: page, time, resources, size, status
	if len(parts) < 6 {
		return hDim.Render(line)
	}

	page := parts[1]
	timeCell := parts[2]
	resCell := parts[3]
	sizeCell := parts[4]
	statusCell := parts[5]

	trimStatus := strings.TrimSpace(statusCell)

	var statusStyled string
	switch trimStatus {
	case "OK", "all ok":
		statusStyled = hGreen.Render(statusCell)
	case "ERROR":
		statusStyled = hRed.Render(statusCell)
	default:
		if strings.Contains(trimStatus, "err") {
			statusStyled = hRed.Render(statusCell)
		} else {
			statusStyled = hMuted.Render(statusCell)
		}
	}

	// Colour page cell — URL gets cyan, aggregate labels get bold
	trimPage := strings.TrimSpace(page)
	var pageStyled string
	if strings.HasPrefix(trimPage, "http") {
		pageStyled = hCyan.Render(page)
	} else if strings.HasPrefix(trimPage, "AVERAGE") || strings.HasPrefix(trimPage, "TOTAL") {
		pageStyled = hBold.Render(page)
	} else {
		pageStyled = hBold.Render(page) // header
	}

	return hMuted.Render("|") +
		pageStyled +
		hMuted.Render("|") +
		hYellow.Render(timeCell) +
		hMuted.Render("|") +
		hDim.Render(resCell) +
		hMuted.Render("|") +
		hGreen.Render(sizeCell) +
		hMuted.Render("|") +
		statusStyled +
		hMuted.Render("|")
}

// colorLabelLine colours "LABEL:   value" lines
func colorLabelLine(line, label string) string {
	idx := strings.Index(line, label)
	if idx < 0 {
		return hDim.Render(line)
	}
	prefix := line[:idx]
	rest := line[idx:]
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return hDim.Render(line)
	}
	key := rest[:colonIdx+1]
	value := rest[colonIdx+1:]

	var valueStyled string
	switch label {
	case "PAGE:":
		valueStyled = hCyan.Render(value)
	case "LOAD TIME:":
		valueStyled = hYellow.Render(value)
	case "RESOURCES:":
		valueStyled = hGreen.Render(value)
	default:
		valueStyled = hDim.Render(value)
	}
	return prefix + hBold.Render(key) + valueStyled
}

// colorResourceRow colours a resource table data row
func colorResourceRow(line string) string {
	// Format: "  TYPE      DURATION   SIZE         START      URL"
	// We parse by splitting on runs of spaces, but preserve alignment by
	// working with fixed field positions based on the format string widths:
	// 2 indent, 10 type, 10 duration, 12 size, 10 start, rest=url
	if len(line) < 44 {
		return hDim.Render(line)
	}

	indent := line[:2]
	rest := line[2:]

	// type: 10 chars
	if len(rest) < 10 {
		return hDim.Render(line)
	}
	typeField := rest[:10]
	rest = rest[10:]

	// duration: 10 chars
	if len(rest) < 10 {
		return hDim.Render(line)
	}
	durationField := rest[:10]
	rest = rest[10:]

	// size: 12 chars
	if len(rest) < 12 {
		return hDim.Render(line)
	}
	sizeField := rest[:12]
	rest = rest[12:]

	// start: 10 chars
	if len(rest) < 10 {
		return hDim.Render(line)
	}
	startField := rest[:10]
	urlField := rest[10:]

	typeName := strings.TrimSpace(typeField)
	typeStyle, ok := typeColors[typeName]
	if !ok {
		typeStyle = hDim
	}

	return indent +
		typeStyle.Render(typeField) +
		hYellow.Render(durationField) +
		hGreen.Render(sizeField) +
		hMuted.Render(startField) +
		hCyan.Render(urlField)
}

// isRepeat checks if a string is entirely made of one rune (ignoring length)
func isRepeat(s string, r rune) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c != r {
			return false
		}
	}
	return true
}
