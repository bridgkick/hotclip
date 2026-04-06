package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/bridgkick/hotclip/internal/model"
)

type fileData struct {
	Version int          `json:"version"`
	Links   []model.Link `json:"links"`
}

// Store manages JSON persistence for links.
type Store struct {
	path string
	data fileData
}

// New loads the store from disk, creating the file and directory if needed.
func New(path string) (*Store, error) {
	s := &Store{path: path, data: fileData{Version: 1}}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			return s, s.save()
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, err
	}
	return s, nil
}

// All returns links sorted by activity.
func (s *Store) All() []model.Link {
	out := make([]model.Link, len(s.data.Links))
	copy(out, s.data.Links)
	model.SortByActivity(out)
	return out
}

// AllUnsorted returns a copy of links without sorting.
func (s *Store) AllUnsorted() []model.Link {
	out := make([]model.Link, len(s.data.Links))
	copy(out, s.data.Links)
	return out
}

// Add inserts a new link and saves.
func (s *Store) Add(link model.Link) error {
	s.data.Links = append(s.data.Links, link)
	return s.save()
}

// Delete removes a link by ID and saves.
func (s *Store) Delete(id string) error {
	for i, l := range s.data.Links {
		if l.ID == id {
			s.data.Links = append(s.data.Links[:i], s.data.Links[i+1:]...)
			return s.save()
		}
	}
	return nil
}

// Update modifies a link's title and URL by ID and saves.
func (s *Store) Update(id, title, url string) error {
	for i, l := range s.data.Links {
		if l.ID == id {
			s.data.Links[i].Title = title
			s.data.Links[i].URL = url
			return s.save()
		}
	}
	return nil
}

// BumpActivity increments use count and updates last used time.
func (s *Store) BumpActivity(id string) error {
	for i, l := range s.data.Links {
		if l.ID == id {
			s.data.Links[i].UseCount = l.UseCount + 1
			s.data.Links[i].LastUsed = time.Now()
			return s.save()
		}
	}
	return nil
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}
