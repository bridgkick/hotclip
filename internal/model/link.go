package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Link represents a saved URL with activity tracking.
type Link struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Tags      []string  `json:"tags,omitempty"`
	UseCount  int       `json:"use_count"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
}

// NewLink creates a Link with a random ID and timestamps set to now.
func NewLink(title, url string, tags []string) Link {
	now := time.Now()
	return Link{
		ID:        randomID(),
		Title:     title,
		URL:       url,
		Tags:      tags,
		UseCount:  0,
		CreatedAt: now,
		LastUsed:  now,
	}
}

func randomID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// list.DefaultItem interface

func (l Link) FilterValue() string {
	parts := []string{l.Title, l.URL}
	parts = append(parts, l.Tags...)
	return strings.Join(parts, " ")
}

// RelativeTime returns a human-friendly relative time string.
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// SortMode controls the list ordering.
type SortMode int

const (
	SortRecent       SortMode = iota // LastUsed desc
	SortMostShared                   // UseCount desc
	SortAlphabetical                 // Title asc
)

// Next cycles to the next sort mode.
func (m SortMode) Next() SortMode { return (m + 1) % 3 }

// Label returns a short display string for the mode.
func (m SortMode) Label() string {
	switch m {
	case SortMostShared:
		return "most shared"
	case SortAlphabetical:
		return "a-z"
	default:
		return "recent"
	}
}

// SortLinks sorts links according to the given mode.
func SortLinks(links []Link, mode SortMode) {
	switch mode {
	case SortMostShared:
		sort.Slice(links, func(i, j int) bool {
			if links[i].UseCount == links[j].UseCount {
				return links[i].LastUsed.After(links[j].LastUsed)
			}
			return links[i].UseCount > links[j].UseCount
		})
	case SortAlphabetical:
		sort.Slice(links, func(i, j int) bool {
			return strings.ToLower(links[i].Title) < strings.ToLower(links[j].Title)
		})
	default:
		SortByActivity(links)
	}
}

// SortByActivity sorts links by LastUsed desc, then UseCount desc.
func SortByActivity(links []Link) {
	sort.Slice(links, func(i, j int) bool {
		if links[i].LastUsed.Equal(links[j].LastUsed) {
			return links[i].UseCount > links[j].UseCount
		}
		return links[i].LastUsed.After(links[j].LastUsed)
	})
}
