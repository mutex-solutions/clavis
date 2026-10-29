package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mutex-solutions/clavis/internal/provider"
)

func models(ids ...string) []provider.ModelInfo {
	out := make([]provider.ModelInfo, len(ids))
	for i, id := range ids {
		out[i] = provider.ModelInfo{ID: id, OwnedBy: "acme"}
	}
	return out
}

func labels(items []selectItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.label
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func teaKey(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func TestBuildSelectItems_NoPins(t *testing.T) {
	items := buildSelectItems(models("a", "b", "c"), "", nil)
	if !eq(labels(items), []string{"a", "b", "c"}) {
		t.Fatalf("order = %v", labels(items))
	}
	for _, it := range items {
		if it.pinned {
			t.Errorf("%q should not be pinned", it.label)
		}
	}
}

func TestBuildSelectItems_PinFloatsToTop(t *testing.T) {
	items := buildSelectItems(models("a", "b", "c"), "", []string{"c"})
	if !eq(labels(items), []string{"c", "a", "b"}) {
		t.Fatalf("order = %v, want [c a b]", labels(items))
	}
	if !items[0].pinned {
		t.Error("first item (c) should be pinned")
	}
	if items[1].pinned || items[2].pinned {
		t.Error("a and b should not be pinned")
	}
}

func TestBuildSelectItems_MultiPinPreservesGivenOrder(t *testing.T) {
	// Most-recent-first is the caller's responsibility; we preserve the slice order.
	items := buildSelectItems(models("a", "b", "c", "d"), "", []string{"c", "a"})
	if !eq(labels(items), []string{"c", "a", "b", "d"}) {
		t.Fatalf("order = %v, want [c a b d]", labels(items))
	}
	if !items[0].pinned || !items[1].pinned {
		t.Error("c and a should be pinned")
	}
}

func TestBuildSelectItems_DedupPins(t *testing.T) {
	items := buildSelectItems(models("a", "b"), "", []string{"a", "a", "b"})
	if !eq(labels(items), []string{"a", "b"}) {
		t.Fatalf("order = %v, want [a b] (no dupes)", labels(items))
	}
}

func TestBuildSelectItems_SkipsUnknownAndEmptyPins(t *testing.T) {
	// "" and a model not in the catalog must be ignored, not inserted.
	items := buildSelectItems(models("a", "b"), "", []string{"", "ghost", "b"})
	if !eq(labels(items), []string{"b", "a"}) {
		t.Fatalf("order = %v, want [b a]", labels(items))
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (ghost/empty dropped)", len(items))
	}
}

func TestBuildSelectItems_ExtraLabelFirst(t *testing.T) {
	items := buildSelectItems(models("a", "b"), "(default — use sonnet)", []string{"b"})
	want := []string{"(default — use sonnet)", "b", "a"}
	if !eq(labels(items), want) {
		t.Fatalf("order = %v, want %v", labels(items), want)
	}
	first := items[0]
	if first.value != "" {
		t.Errorf("extra entry value = %q, want empty (signals fallback)", first.value)
	}
	if first.pinned {
		t.Error("extra entry should not be pinned")
	}
}

func TestApplyFilter_ResetsCursorAndOffset(t *testing.T) {
	m := newModelSelect("t", "h", buildSelectItems(models("alpha", "bravo", "charlie", "delta", "echo"), "", nil), 3)
	// Move the cursor/offset down as if the user scrolled.
	m.cursor = 4
	m.offset = 2
	// Typing a filter must snap back to the top.
	m.input.SetValue("a") // matches alpha, bravo(?), charlie, delta — contains 'a'
	m.applyFilter()
	if m.cursor != 0 {
		t.Errorf("cursor = %d after filter, want 0", m.cursor)
	}
	if m.offset != 0 {
		t.Errorf("offset = %d after filter, want 0", m.offset)
	}
}

func TestApplyFilter_MatchesLabelSubstring(t *testing.T) {
	m := newModelSelect("t", "h", buildSelectItems(models("gpt-4o", "claude-opus", "deepseek"), "", nil), 5)
	m.input.SetValue("opus")
	m.applyFilter()
	if len(m.filtered) != 1 || m.filtered[0].label != "claude-opus" {
		t.Fatalf("filtered = %v, want [claude-opus]", labels(m.filtered))
	}
}

func TestApplyFilter_EmptyQueryShowsAll(t *testing.T) {
	all := buildSelectItems(models("a", "b", "c"), "", nil)
	m := newModelSelect("t", "h", all, 5)
	m.input.SetValue("zzz")
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Fatalf("expected no matches for zzz, got %v", labels(m.filtered))
	}
	m.input.SetValue("")
	m.applyFilter()
	if len(m.filtered) != 3 {
		t.Fatalf("empty query should show all 3, got %d", len(m.filtered))
	}
}

func TestUpdate_EscSetsAborted(t *testing.T) {
	m := newModelSelect("t", "h", buildSelectItems(models("a", "b"), "", nil), 5)
	nm, _ := m.Update(teaKey(tea.KeyEsc))
	got := nm.(modelSelect)
	if !got.aborted || got.interrupted {
		t.Errorf("esc: aborted=%v interrupted=%v, want aborted=true", got.aborted, got.interrupted)
	}
}

func TestUpdate_CtrlCSetsInterrupted(t *testing.T) {
	m := newModelSelect("t", "h", buildSelectItems(models("a", "b"), "", nil), 5)
	nm, _ := m.Update(teaKey(tea.KeyCtrlC))
	got := nm.(modelSelect)
	if !got.interrupted || got.aborted {
		t.Errorf("ctrl+c: aborted=%v interrupted=%v, want interrupted=true", got.aborted, got.interrupted)
	}
}

func TestUpdate_DownThenUpMovesCursor(t *testing.T) {
	m := newModelSelect("t", "h", buildSelectItems(models("a", "b", "c"), "", nil), 5)
	nm, _ := m.Update(teaKey(tea.KeyDown))
	m2 := nm.(modelSelect)
	if m2.cursor != 1 {
		t.Fatalf("after down cursor=%d, want 1", m2.cursor)
	}
	nm, _ = m2.Update(teaKey(tea.KeyUp))
	m3 := nm.(modelSelect)
	if m3.cursor != 0 {
		t.Fatalf("after up cursor=%d, want 0", m3.cursor)
	}
}

func TestUpdate_DownStopsAtBottom(t *testing.T) {
	m := newModelSelect("t", "h", buildSelectItems(models("a", "b"), "", nil), 5)
	for i := 0; i < 5; i++ {
		nm, _ := m.Update(teaKey(tea.KeyDown))
		m = nm.(modelSelect)
	}
	if m.cursor != 1 {
		t.Errorf("cursor=%d, want 1 (clamped at last of 2 items)", m.cursor)
	}
}

func TestUpdate_ScrollOffsetTracksCursor(t *testing.T) {
	// 6 items, viewport height 3: scrolling down past the window must advance offset.
	m := newModelSelect("t", "h", buildSelectItems(models("a", "b", "c", "d", "e", "f"), "", nil), 3)
	for i := 0; i < 3; i++ { // cursor 0->3
		nm, _ := m.Update(teaKey(tea.KeyDown))
		m = nm.(modelSelect)
	}
	if m.cursor != 3 {
		t.Fatalf("cursor=%d, want 3", m.cursor)
	}
	if m.offset != 1 { // window [1..3] so 3 is visible
		t.Errorf("offset=%d, want 1 (viewport followed cursor)", m.offset)
	}
}

func TestBuildSelectItems_ShowsContextWindow(t *testing.T) {
	window := 1048576
	items := buildSelectItems([]provider.ModelInfo{
		{ID: "big", OwnedBy: "acme", MaxInputTokens: &window},
		{ID: "unknown", OwnedBy: "acme"},
	}, "", nil)

	if items[0].owner != "acme · 1M" {
		t.Errorf("owner = %q, want the reported window appended", items[0].owner)
	}
	if items[1].owner != "acme" {
		t.Errorf("owner = %q, want no window suffix when none is reported", items[1].owner)
	}
}

func TestProviderGroupSplitsGenericOwners(t *testing.T) {
	cases := []struct {
		model provider.ModelInfo
		want  string
	}{
		{provider.ModelInfo{ID: "k3.1-pro", OwnedBy: "anthropic"}, "Kimi"},
		{provider.ModelInfo{ID: "kimi-for-coding", OwnedBy: "anthropic"}, "Kimi"},
		{provider.ModelInfo{ID: "deepseek-flash", OwnedBy: "anthropic"}, "DeepSeek"},
		{provider.ModelInfo{ID: "glm-5.3", OwnedBy: "anthropic"}, "GLM"},
		{provider.ModelInfo{ID: "kimi-k3-opencode", OwnedBy: "opencode-go"}, "OpenCode Go"},
		{provider.ModelInfo{ID: "gemini-3.8-flash-high", OwnedBy: "antigravity"}, "Antigravity"},
	}
	for _, c := range cases {
		if got := providerGroup(c.model); got != c.want {
			t.Errorf("providerGroup(%s, owner=%s) = %q, want %q", c.model.ID, c.model.OwnedBy, got, c.want)
		}
	}
}

func TestBuildSelectItemsGroupsByProvider(t *testing.T) {
	items := buildSelectItems([]provider.ModelInfo{
		{ID: "glm-5.3", OwnedBy: "anthropic"},
		{ID: "deepseek-flash", OwnedBy: "anthropic"},
		{ID: "k3.1-pro", OwnedBy: "anthropic"},
		{ID: "gemini-3.8-flash-high", OwnedBy: "antigravity"},
	}, "", nil)

	wantGroups := []string{"Antigravity", "DeepSeek", "GLM", "Kimi"}
	for i, want := range wantGroups {
		if items[i].group != want {
			t.Errorf("item %d group = %q, want %q", i, items[i].group, want)
		}
	}
}

func TestModelSelectViewShowsGroupHeadings(t *testing.T) {
	items := buildSelectItems([]provider.ModelInfo{
		{ID: "glm-5.3", OwnedBy: "anthropic"},
		{ID: "deepseek-flash", OwnedBy: "anthropic"},
		{ID: "k3.1-pro", OwnedBy: "anthropic"},
		{ID: "gemini-3.8-flash-high", OwnedBy: "antigravity"},
	}, "", nil)
	view := newModelSelect("Models", "hint", items, 10).View()
	for _, heading := range []string{"Antigravity", "DeepSeek", "GLM", "Kimi"} {
		if !strings.Contains(view, heading) {
			t.Errorf("view is missing the %q heading:\n%s", heading, view)
		}
	}
}
