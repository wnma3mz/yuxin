package app

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Accent colors use restrained 256-color values for broad terminal support.
// Body text and borders intentionally inherit the terminal foreground color,
// which keeps them readable on both dark and light user themes.
const (
	ansiAmber       = "38;5;214"
	ansiEmerald     = "38;5;36"
	ansiEmeraldSoft = "38;5;29"
	ansiSky         = "38;5;74"
)

func panel(title string, rows []string, width int) []string {
	titleText := " " + title + " "
	top := "╭─" + titleText + strings.Repeat("─", max(0, width-3-displayWidth(titleText))) + "╮"
	result := []string{top}
	for _, row := range rows {
		result = append(result, "│ "+pad(truncate(row, width-4), width-4, alignLeft)+" │")
	}
	return append(result, "╰"+strings.Repeat("─", width-2)+"╯")
}

func joinPanels(left, right []string) []string {
	height := max(len(left), len(right))
	left = extendPanel(left, height)
	right = extendPanel(right, height)
	leftWidth := 0
	for _, line := range left {
		leftWidth = max(leftWidth, displayWidth(line))
	}
	result := make([]string, 0, height)
	for index := 0; index < height; index++ {
		leftLine := ""
		if index < len(left) {
			leftLine = left[index]
		}
		rightLine := ""
		if index < len(right) {
			rightLine = right[index]
		}
		result = append(result, pad(leftLine, leftWidth, alignLeft)+" "+rightLine)
	}
	return result
}

func extendPanel(lines []string, height int) []string {
	if len(lines) < 2 || len(lines) >= height {
		return lines
	}
	width := displayWidth(lines[0])
	result := append([]string{}, lines[:len(lines)-1]...)
	for len(result) < height-1 {
		result = append(result, "│"+strings.Repeat(" ", max(0, width-2))+"│")
	}
	return append(result, lines[len(lines)-1])
}

func metric(label, value string, width int) string {
	return pad(label, max(displayWidth(label)+1, width-displayWidth(value)), alignLeft) + value
}

func threeColumns(left, center, right string, width int) string {
	leftWidth := width / 3
	rightWidth := width / 3
	centerWidth := width - leftWidth - rightWidth
	return pad(truncate(left, leftWidth), leftWidth, alignLeft) +
		pad(truncate(center, centerWidth), centerWidth, alignCenter) +
		pad(truncate(right, rightWidth), rightWidth, alignRight)
}

func progressBar(progress float64, width int, useColor bool) string {
	return progressBarWithColor(progress, width, useColor, ansiSky)
}

func progressBarWithColor(progress float64, width int, useColor bool, filledColor string) string {
	filled := int(math.Round(clampFloat(progress, 0, 1) * float64(width)))
	return color(strings.Repeat("█", filled), filledColor, useColor) +
		strings.Repeat("░", width-filled)
}

func money(value float64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	whole := int64(value)
	fraction := int(math.Round((value - float64(whole)) * 100))
	if fraction == 100 {
		whole++
		fraction = 0
	}
	return fmt.Sprintf("%s¥%s.%02d", sign, commaInt64(whole), fraction)
}

func commaInt(value int) string { return commaInt64(int64(value)) }

func commaInt64(value int64) string {
	sign := ""
	if value < 0 {
		sign, value = "-", -value
	}
	digits := fmt.Sprintf("%d", value)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "," + digits[index:]
	}
	return sign + digits
}

func duration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	hours, rest := seconds/3600, seconds%3600
	minutes, secs := rest/60, rest%60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func clock(seconds int) string {
	return fmt.Sprintf("%02d:%02d", seconds/3600, (seconds%3600)/60)
}

type alignment int

const (
	alignLeft alignment = iota
	alignCenter
	alignRight
)

func pad(value string, width int, alignment alignment) string {
	missing := max(0, width-displayWidth(value))
	switch alignment {
	case alignRight:
		return strings.Repeat(" ", missing) + value
	case alignCenter:
		left := missing / 2
		return strings.Repeat(" ", left) + value + strings.Repeat(" ", missing-left)
	default:
		return value + strings.Repeat(" ", missing)
	}
}

func truncate(value string, width int) string {
	if displayWidth(value) <= width {
		return value
	}
	plain := ansiPattern.ReplaceAllString(value, "")
	var result strings.Builder
	used := 0
	for _, char := range plain {
		charWidth := runeWidth(char)
		if used+charWidth > max(0, width-1) {
			break
		}
		result.WriteRune(char)
		used += charWidth
	}
	return result.String() + "…"
}

func displayWidth(value string) int {
	plain := ansiPattern.ReplaceAllString(value, "")
	width := 0
	for _, char := range plain {
		width += runeWidth(char)
	}
	return width
}

func runeWidth(char rune) int {
	if char == '\u200d' || unicode.Is(unicode.Mn, char) || unicode.Is(unicode.Me, char) {
		return 0
	}
	if char >= 0x1100 && (char <= 0x115f || char == 0x2329 || char == 0x232a || char == 0x23f3 ||
		(char >= 0x2e80 && char <= 0xa4cf) || (char >= 0xac00 && char <= 0xd7a3) ||
		(char >= 0xf900 && char <= 0xfaff) || (char >= 0xfe10 && char <= 0xfe6f) ||
		(char >= 0xff00 && char <= 0xff60) || (char >= 0x1f300 && char <= 0x1faff)) {
		return 2
	}
	if char == utf8.RuneError {
		return 1
	}
	return 1
}

func color(value, code string, enabled bool) string {
	if !enabled {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func clampFloat(value, low, high float64) float64 {
	return math.Min(high, math.Max(low, value))
}
