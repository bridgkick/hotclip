package ui

import "charm.land/bubbles/v2/key"

type listKeyMap struct {
	Copy   key.Binding
	Open   key.Binding
	Add    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Sort   key.Binding
}

var listKeys = listKeyMap{
	Copy: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "copy link"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add link"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Sort: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sort"),
	),
}

func (k listKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Copy, k.Open, k.Add, k.Edit, k.Delete, k.Sort}
}

func (k listKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
