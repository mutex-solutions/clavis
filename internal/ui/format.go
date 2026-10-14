package ui

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatTokens renders a token count the way people say it out loud:
// 128000 → "128K", 1000000 → "1M", 262144 → "256K".
//
// Providers report both decimal (128000) and binary (131072, 262144) round
// numbers, and both are usually spoken in the shorter form. So an exact
// division by 1000 wins, and an exact division by 1024 is the fallback —
// 128000 reads as "128K" while 262144 reads as "256K" rather than "262.1K".
// Anything that divides evenly by neither keeps one decimal.
//
// Zero and negatives return "" so callers can treat "unset" as "print nothing".
func FormatTokens(n int) string {
	switch {
	case n <= 0:
		return ""
	case n < 1000:
		return strconv.Itoa(n)
	case n < 1_000_000:
		if v, ok := exactScale(n, 1000); ok {
			return strconv.Itoa(v) + "K"
		}
		if v, ok := exactScale(n, 1024); ok {
			return strconv.Itoa(v) + "K"
		}
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		if v, ok := exactScale(n, 1_000_000); ok {
			return strconv.Itoa(v) + "M"
		}
		if v, ok := exactScale(n, 1_048_576); ok {
			return strconv.Itoa(v) + "M"
		}
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}

// exactScale divides n by unit, reporting whether it divided evenly.
func exactScale(n, unit int) (int, bool) {
	if n%unit != 0 {
		return 0, false
	}
	return n / unit, true
}

// ParseTokenCount reads a token count in the spellings Claude Code's own
// /autocompact accepts, so the numbers here feel the same as the ones there:
//
//	"128000" → 128000    a plain count
//	"128k"   → 128000    k/K and m/M suffixes
//	"1.5M"   → 1500000   a fractional multiplier
//	"200"    → 200000    a bare 100–1000 means thousands
//
// Digit separators ("128_000", "128,000") are accepted. Anything else, or a
// non-positive result, is an error.
func ParseTokenCount(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("required")
	}

	mult := 1
	switch s[len(s)-1] {
	case 'k', 'K':
		mult, s = 1000, strings.TrimSpace(s[:len(s)-1])
	case 'm', 'M':
		mult, s = 1_000_000, strings.TrimSpace(s[:len(s)-1])
	}
	if s == "" {
		return 0, fmt.Errorf("missing a number before the unit")
	}

	clean := strings.NewReplacer(",", "", "_", "").Replace(s)
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%q is not a positive number", strings.TrimSpace(s))
	}

	n := int(v * float64(mult))
	// A bare 200 means 200K — the same shorthand /autocompact uses. Without
	// this, typing "200" would declare a 200-token window and compact instantly.
	if mult == 1 && n >= 100 && n <= 1000 {
		n *= 1000
	}
	if n <= 0 {
		return 0, fmt.Errorf("%q is too small to be a context window", strings.TrimSpace(s))
	}
	return n, nil
}
