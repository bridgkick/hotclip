package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bridgkick/hotclip/internal/model"
)

func TestStoreOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sub", "links.json")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if len(s.All()) != 0 {
		t.Errorf("expected empty store, got %d links", len(s.All()))
	}

	l1 := model.NewLink("Google", "https://google.com", []string{"search"})
	l2 := model.NewLink("GitHub", "https://github.com", []string{"code"})

	if err := s.Add(l1); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := s.Add(l2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if len(s.All()) != 2 {
		t.Errorf("expected 2 links, got %d", len(s.All()))
	}

	// Test Update
	if err := s.Update(l1.ID, "Google Search", "https://www.google.com"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	links := s.AllUnsorted()
	var found *model.Link
	for _, l := range links {
		if l.ID == l1.ID {
			found = &l
			break
		}
	}
	if found == nil || found.Title != "Google Search" || found.URL != "https://www.google.com" {
		t.Errorf("link update not reflected: %+v", found)
	}

	// Test BumpActivity
	beforeBump := time.Now().Add(-1 * time.Second)
	if err := s.BumpActivity(l1.ID); err != nil {
		t.Fatalf("BumpActivity failed: %v", err)
	}
	links = s.AllUnsorted()
	for _, l := range links {
		if l.ID == l1.ID {
			if l.UseCount != 1 {
				t.Errorf("expected UseCount 1, got %d", l.UseCount)
			}
			if l.LastUsed.Before(beforeBump) {
				t.Errorf("expected LastUsed updated")
			}
			break
		}
	}

	// Test persistence by reloading from disk
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if len(s2.All()) != 2 {
		t.Errorf("expected 2 links in reloaded store, got %d", len(s2.All()))
	}

	// Test Delete
	if err := s.Delete(l1.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(s.All()) != 1 {
		t.Errorf("expected 1 link after delete, got %d", len(s.All()))
	}
}
