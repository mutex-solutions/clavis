package ui

import "testing"

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, ""},  // unset — callers print nothing rather than "0"
		{-1, ""}, // never a real window
		{999, "999"},
		{1000, "1K"},
		{128000, "128K"}, // decimal round number, DeepSeek/Qwen
		{131072, "128K"}, // the same window as a power of two
		{200000, "200K"}, // Claude Code's assumption
		{262144, "256K"}, // 262.1K would be unreadable
		{210000, "210K"},
		{128500, "128.5K"}, // neither scaling divides evenly
		{1000000, "1M"},
		{1048576, "1M"},
		{1500000, "1.5M"},
	}

	for _, c := range cases {
		if got := FormatTokens(c.in); got != c.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseTokenCount(t *testing.T) {
	ok := []struct {
		in   string
		want int
	}{
		{"128000", 128000},
		{" 128000 ", 128000},
		{"128k", 128000},
		{"128K", 128000},
		{"1m", 1000000},
		{"1M", 1000000},
		{"1.5M", 1500000},
		{"128,000", 128000},
		{"128_000", 128000},
		// A bare 100–1000 is thousands, matching Claude Code's /autocompact.
		{"200", 200000},
		{"1000", 1000000},
		// Below 100 there is no shorthand to apply; the caller range-checks.
		{"50", 50},
	}
	for _, c := range ok {
		got, err := ParseTokenCount(c.in)
		if err != nil {
			t.Errorf("ParseTokenCount(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseTokenCount(%q) = %d, want %d", c.in, got, c.want)
		}
	}

	bad := []string{"", "   ", "abc", "0", "-5", "k", "1x", "1.5.2M"}
	for _, in := range bad {
		if got, err := ParseTokenCount(in); err == nil {
			t.Errorf("ParseTokenCount(%q) = %d, want an error", in, got)
		}
	}
}

// TestTokenCountRoundTrip pins the invariant the picker relies on: once a
// window has been shown to the user, re-entering that same label must not
// change the number. Otherwise a profile would silently drift its own displayed
// window every time someone retyped what they saw.
//
// Note the asymmetry this deliberately allows: 262144 displays as "256K"
// because that is how the number is spoken, but "256K" is 256000. The exact
// value is never lost — picker options carry the raw count, not the label — so
// the only requirement is that the *label* is a fixed point.
func TestTokenCountRoundTrip(t *testing.T) {
	for _, n := range []int{128000, 200000, 256000, 262144, 512000, 1000000} {
		label := FormatTokens(n)
		parsed, err := ParseTokenCount(label)
		if err != nil {
			t.Errorf("ParseTokenCount(%q) error: %v", label, err)
			continue
		}
		if got := FormatTokens(parsed); got != label {
			t.Errorf("label %q parsed to %d, which displays as %q — the label is not stable",
				label, parsed, got)
		}
	}
}
