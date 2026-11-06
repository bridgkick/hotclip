package model

import (
	"testing"
	"time"
)

func TestNewLink(t *testing.T) {
	l := NewLink("GitHub", "https://github.com", []string{"dev", "git"})
	if l.Title != "GitHub" {
		t.Errorf("expected title GitHub, got %s", l.Title)
	}
	if l.URL != "https://github.com" {
		t.Errorf("expected URL https://github.com, got %s", l.URL)
	}
	if len(l.Tags) != 2 || l.Tags[0] != "dev" || l.Tags[1] != "git" {
		t.Errorf("expected tags [dev, git], got %v", l.Tags)
	}
	if l.ID == "" {
		t.Errorf("expected non-empty ID")
	}
	if l.UseCount != 0 {
		t.Errorf("expected initial UseCount 0, got %d", l.UseCount)
	}
	if l.CreatedAt.IsZero() || l.LastUsed.IsZero() {
		t.Errorf("expected non-zero timestamps")
	}
}

func TestFilterValue(t *testing.T) {
	tests := []struct {
		name     string
		link     Link
		expected string
	}{
		{
			name:     "with tags",
			link:     Link{Title: "GitHub", URL: "https://github.com", Tags: []string{"code", "git"}},
			expected: "GitHub https://github.com code git",
		},
		{
			name:     "without tags",
			link:     Link{Title: "Linear", URL: "https://linear.app"},
			expected: "Linear https://linear.app",
		},
		{
			name:     "empty tags slice",
			link:     Link{Title: "Hacker News", URL: "https://news.ycombinator.com", Tags: []string{}},
			expected: "Hacker News https://news.ycombinator.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.link.FilterValue()
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		time     time.Time
		expected string
	}{
		{now.Add(-10 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-48 * time.Hour), "2d ago"},
	}

	for _, tc := range tests {
		got := RelativeTime(tc.time)
		if got != tc.expected {
			t.Errorf("for time offset %v: expected %q, got %q", time.Since(tc.time), tc.expected, got)
		}
	}
}

func TestSortLinks(t *testing.T) {
	now := time.Now()
	l1 := Link{ID: "1", Title: "Banana", UseCount: 5, LastUsed: now.Add(-1 * time.Hour)}
	l2 := Link{ID: "2", Title: "Apple", UseCount: 10, LastUsed: now.Add(-2 * time.Hour)}
	l3 := Link{ID: "3", Title: "Cherry", UseCount: 5, LastUsed: now.Add(-10 * time.Minute)}

	// Alphabetical
	links := []Link{l1, l2, l3}
	SortLinks(links, SortAlphabetical)
	if links[0].Title != "Apple" || links[1].Title != "Banana" || links[2].Title != "Cherry" {
		t.Errorf("alphabetical sort failed: got %s, %s, %s", links[0].Title, links[1].Title, links[2].Title)
	}

	// Most shared
	links = []Link{l1, l2, l3}
	SortLinks(links, SortMostShared)
	if links[0].ID != "2" || links[1].ID != "3" || links[2].ID != "1" {
		t.Errorf("most shared sort failed: got IDs %s, %s, %s", links[0].ID, links[1].ID, links[2].ID)
	}

	// Recent (by activity: LastUsed desc, then UseCount desc)
	links = []Link{l1, l2, l3}
	SortLinks(links, SortRecent)
	if links[0].ID != "3" || links[1].ID != "1" || links[2].ID != "2" {
		t.Errorf("recent sort failed: got IDs %s, %s, %s", links[0].ID, links[1].ID, links[2].ID)
	}
}

func TestSortModeCycle(t *testing.T) {
	mode := SortRecent
	if mode.Label() != "recent" {
		t.Errorf("expected 'recent', got %q", mode.Label())
	}
	mode = mode.Next()
	if mode != SortMostShared || mode.Label() != "most shared" {
		t.Errorf("expected 'most shared', got %q", mode.Label())
	}
	mode = mode.Next()
	if mode != SortAlphabetical || mode.Label() != "a-z" {
		t.Errorf("expected 'a-z', got %q", mode.Label())
	}
	mode = mode.Next()
	if mode != SortRecent {
		t.Errorf("expected wrap around to SortRecent, got %v", mode)
	}
}
