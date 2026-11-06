package ui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/bridgkick/hotclip/internal/model"
	"github.com/bridgkick/hotclip/internal/store"
)

// runFilterCmds executes commands produced by tea.Model up to FilterMatchesMsg.
func runFilterCmds(m tea.Model, cmd tea.Cmd) tea.Model {
	if cmd == nil {
		return m
	}
	ch := make(chan tea.Msg, 1)
	go func() {
		ch <- cmd()
	}()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(15 * time.Millisecond):
		return m
	}

	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runFilterCmds(m, c)
		}
		return m
	}
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		m, nextCmd := m.Update(msg)
		return runFilterCmds(m, nextCmd)
	}
	return m
}

func setupTestApp(t *testing.T) (*store.Store, Model) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "links.json")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	s.Add(model.NewLink("GitHub", "https://github.com", []string{"code", "git"}))
	s.Add(model.NewLink("Linear", "https://linear.app", []string{"issues", "pm"}))
	s.Add(model.NewLink("Hacker News", "https://news.ycombinator.com", []string{"news"}))
	s.Add(model.NewLink("Documentation", "https://docs.example.org", []string{"docs"}))

	m := New(s)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return s, updated.(Model)
}

func applyFilter(m tea.Model, query string) tea.Model {
	// Press '/' to start filtering
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = runFilterCmds(m, cmd)

	// Type characters
	for _, c := range query {
		m, cmd = m.Update(tea.KeyPressMsg{Code: rune(c), Text: string(c)})
		m = runFilterCmds(m, cmd)
	}

	// Press Enter to accept filter
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = runFilterCmds(m, cmd)
	return m
}

// TestFilterOutputClean verifies that filtering produces no broken ANSI escape sequences.
func TestFilterOutputClean(t *testing.T) {
	queries := []string{"git", "com", "code", "issues", "linear", "org", "docs", "news"}

	for _, q := range queries {
		t.Run("query_"+q, func(t *testing.T) {
			_, m := setupTestApp(t)

			var cmd tea.Cmd
			var model tea.Model = m
			model, cmd = model.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
			model = runFilterCmds(model, cmd)

			for _, c := range q {
				model, cmd = model.Update(tea.KeyPressMsg{Code: rune(c), Text: string(c)})
				model = runFilterCmds(model, cmd)
			}

			view := model.(Model).View().Content

			// Strip all well-formed ANSI sequences: \x1b\[[0-9;]*[a-zA-Z] or \x1b\]...\x07
			ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
			stripped := ansiRegex.ReplaceAllString(view, "")

			// Check for leftover broken color escape fragments like ";102m" or "38;2;102m"
			brokenRegex := regexp.MustCompile(`[0-9;]+m`)
			if match := brokenRegex.FindString(stripped); match != "" {
				t.Errorf("query %q produced broken ANSI escape characters in output: %q\nFull view:\n%s", q, match, view)
			}
		})
	}
}

// TestFilterCopyPreservesFilter verifies that copying an item while filtered does not wipe visible items.
func TestFilterCopyPreservesFilter(t *testing.T) {
	_, m := setupTestApp(t)
	filtered := applyFilter(m, "git")

	appModel := filtered.(Model)
	if len(appModel.list.VisibleItems()) != 1 {
		t.Fatalf("expected 1 visible item, got %d", len(appModel.list.VisibleItems()))
	}

	// Press Enter to copy
	copied, cmd := filtered.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	copied = runFilterCmds(copied, cmd)

	appModel = copied.(Model)
	visible := len(appModel.list.VisibleItems())
	if visible != 1 {
		t.Fatalf("expected 1 visible item preserved after copy, got %d", visible)
	}

	selected := appModel.list.SelectedItem()
	if selected == nil || selected.(linkItem).Link.Title != "GitHub" {
		t.Fatalf("expected GitHub selected, got %v", selected)
	}
}

// TestFilterSelectByIDAlphabetical ensures that in A-Z sort mode, selectByID keeps selection valid.
func TestFilterSelectByIDAlphabetical(t *testing.T) {
	_, m := setupTestApp(t)

	// Cycle sort to A-Z: recent -> most shared -> a-z
	var model tea.Model = m
	model, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})

	filtered := applyFilter(model, "linear")
	appModel := filtered.(Model)
	if len(appModel.list.VisibleItems()) != 1 {
		t.Fatalf("expected 1 visible item, got %d", len(appModel.list.VisibleItems()))
	}

	// Copy selected
	copied, cmd := filtered.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	copied = runFilterCmds(copied, cmd)

	appModel = copied.(Model)
	selected := appModel.list.SelectedItem()
	if selected == nil {
		t.Fatalf("selected item is nil after copy in A-Z mode")
	}
	if selected.(linkItem).Link.Title != "Linear" {
		t.Errorf("expected selected Linear, got %s", selected.(linkItem).Link.Title)
	}
}

// TestFilterCycleSort verifies cycling sort mode while filtered updates title and keeps items.
func TestFilterCycleSort(t *testing.T) {
	_, m := setupTestApp(t)
	// Filter for "https" which matches all 4 links
	filtered := applyFilter(m, "https")

	appModel := filtered.(Model)
	if len(appModel.list.VisibleItems()) != 4 {
		t.Fatalf("expected 4 visible items, got %d", len(appModel.list.VisibleItems()))
	}

	// Cycle sort with 's'
	sorted, _ := filtered.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	appModel = sorted.(Model)
	if len(appModel.list.VisibleItems()) != 4 {
		t.Errorf("expected 4 visible items after sort, got %d", len(appModel.list.VisibleItems()))
	}
	if appModel.sortMode != model.SortMostShared {
		t.Errorf("expected SortMostShared, got %v", appModel.sortMode)
	}
	if !strings.Contains(appModel.list.Title, "most shared") {
		t.Errorf("expected title to show most shared, got %s", appModel.list.Title)
	}
}

// TestFilterDelete verifies that deleting a filtered item preserves remaining filtered results.
func TestFilterDelete(t *testing.T) {
	_, m := setupTestApp(t)
	// Filter for "git" which matches GitHub (via Title, URL, and tags)
	filtered := applyFilter(m, "git")

	appModel := filtered.(Model)
	if len(appModel.list.VisibleItems()) != 1 {
		t.Fatalf("expected 1 visible item, got %d", len(appModel.list.VisibleItems()))
	}

	// Press 'd' to initiate delete
	delPrompt, _ := filtered.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	appModel = delPrompt.(Model)
	if !appModel.confirmingDel {
		t.Fatalf("expected confirmingDel to be true")
	}

	// Confirm with 'y'
	deleted, _ := delPrompt.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	appModel = deleted.(Model)

	// Since only 1 matched and was deleted, visible items should now be 0
	if len(appModel.list.VisibleItems()) != 0 {
		t.Errorf("expected 0 visible items after deleting only match, got %d", len(appModel.list.VisibleItems()))
	}

	// Now press Esc to clear filter -> remaining 3 items should appear
	cleared, _ := deleted.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	appModel = cleared.(Model)
	if len(appModel.list.VisibleItems()) != 3 {
		t.Errorf("expected 3 items after clearing filter, got %d", len(appModel.list.VisibleItems()))
	}
}

// TestFilterEscClearsFilter verifies that pressing Esc in FilterApplied state returns to all items.
func TestFilterEscClearsFilter(t *testing.T) {
	_, m := setupTestApp(t)
	filtered := applyFilter(m, "linear")

	appModel := filtered.(Model)
	if len(appModel.list.VisibleItems()) != 1 {
		t.Fatalf("expected 1 visible item, got %d", len(appModel.list.VisibleItems()))
	}

	// Press Esc to clear filter
	cleared, _ := filtered.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	appModel = cleared.(Model)
	if len(appModel.list.VisibleItems()) != 4 {
		t.Errorf("expected 4 visible items after Esc, got %d", len(appModel.list.VisibleItems()))
	}
}

// TestFilterMouseClickToCopy verifies left-click row copying when a filter is applied.
func TestFilterMouseClickToCopy(t *testing.T) {
	_, m := setupTestApp(t)
	filtered := applyFilter(m, "linear")

	appModel := filtered.(Model)
	if len(appModel.list.VisibleItems()) != 1 {
		t.Fatalf("expected 1 visible item, got %d", len(appModel.list.VisibleItems()))
	}

	// Row 0 is at headerRows (4)
	click := tea.MouseClickMsg{
		X:      10,
		Y:      4,
		Button: tea.MouseLeft,
	}
	clicked, cmd := filtered.Update(click)
	clicked = runFilterCmds(clicked, cmd)

	appModel = clicked.(Model)
	if len(appModel.list.VisibleItems()) != 1 {
		t.Errorf("expected 1 visible item after mouse click, got %d", len(appModel.list.VisibleItems()))
	}
	selected := appModel.list.SelectedItem()
	if selected == nil || selected.(linkItem).Link.Title != "Linear" {
		t.Errorf("expected Linear selected, got %v", selected)
	}
}

// TestFilterAddLinkResetsFilter verifies adding a link resets the filter so new item is selected.
func TestFilterAddLinkResetsFilter(t *testing.T) {
	s, m := setupTestApp(t)
	filtered := applyFilter(m, "git")

	// Press 'a' to enter add form
	formModel, _ := filtered.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	app := formModel.(Model)
	if app.view != viewForm {
		t.Fatalf("expected viewForm, got %v", app.view)
	}

	// Fill URL and Title
	app.urlInput.SetValue("https://test.com")
	app.titleInput.SetValue("Test Link")

	// Submit with Enter
	submitted, _ := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	app = submitted.(Model)

	if app.view != viewList {
		t.Fatalf("expected viewList after submit, got %v", app.view)
	}

	// Filter should be reset so all 5 links are visible
	if len(app.list.VisibleItems()) != 5 {
		t.Errorf("expected 5 visible items after adding new link, got %d", len(app.list.VisibleItems()))
	}

	selected := app.list.SelectedItem()
	if selected == nil || selected.(linkItem).Link.Title != "Test Link" {
		t.Errorf("expected Test Link selected, got %v", selected)
	}

	// Verify store has 5 links
	if len(s.All()) != 5 {
		t.Errorf("expected store to have 5 links, got %d", len(s.All()))
	}
}
