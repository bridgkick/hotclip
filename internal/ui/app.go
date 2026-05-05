package ui

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bridgkick/hotclip/internal/clipboard"
	"github.com/bridgkick/hotclip/internal/model"
	"github.com/bridgkick/hotclip/internal/store"
	"github.com/pkg/browser"
	"golang.org/x/net/html"
)

type viewState int

const (
	viewList viewState = iota
	viewForm
)

// titleFetchedMsg carries the result of an async page title lookup.
type titleFetchedMsg struct {
	title string
}

// fetchPageTitle fetches the <title> from a URL.
func fetchPageTitle(url string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return titleFetchedMsg{}
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; hotclip/1.0)")
		resp, err := client.Do(req)
		if err != nil {
			return titleFetchedMsg{}
		}
		defer resp.Body.Close()

		// Read at most 64KB to find the title.
		body := io.LimitReader(resp.Body, 64*1024)
		title := extractTitle(body)
		if isJunkTitle(title) {
			return titleFetchedMsg{}
		}
		return titleFetchedMsg{title: title}
	}
}

// extractTitle parses HTML and returns the content of the first <title> tag.
func extractTitle(r io.Reader) string {
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return ""
		case html.StartTagToken:
			tn, _ := z.TagName()
			if string(tn) == "title" {
				if z.Next() == html.TextToken {
					return strings.TrimSpace(string(z.Text()))
				}
				return ""
			}
		}
	}
}

// isJunkTitle returns true for bot-check / placeholder page titles.
func isJunkTitle(title string) bool {
	lower := strings.ToLower(title)
	junk := []string{
		"just a moment",
		"attention required",
		"access denied",
		"please wait",
		"checking your browser",
		"one moment",
		"verify you are human",
		"security check",
	}
	for _, j := range junk {
		if strings.Contains(lower, j) {
			return true
		}
	}
	return title == ""
}

// linkItem wraps model.Link to implement list.DefaultItem.
type linkItem struct {
	model.Link
}

func (i linkItem) Title() string {
	var badge string
	if i.UseCount > 0 {
		badge = fmt.Sprintf("  %s", badgeStyle.Render(fmt.Sprintf("%dx, %s", i.UseCount, model.RelativeTime(i.LastUsed))))
	} else {
		badge = "  " + dimStyle.Render("●")
	}
	url := i.URL
	if len([]rune(url)) > 32 {
		url = string([]rune(url)[:32]) + "…"
	}
	title := i.Link.Title
	if len([]rune(title)) > 53 {
		title = string([]rune(title)[:53]) + "…"
	}
	return title + "  " + dimStyle.Render(url) + badge
}
func (i linkItem) Description() string  { return "" }
func (i linkItem) FilterValue() string  { return i.Link.FilterValue() }

// Model is the root bubbletea model.
type Model struct {
	store      *store.Store
	list       list.Model
	view       viewState
	titleInput textinput.Model
	urlInput   textinput.Model
	focusTitle    bool
	editingID     string // non-empty when editing an existing link
	confirmingDel bool   // waiting for delete confirmation
	sortMode      model.SortMode
	width      int
	height     int
}

// New creates the root model wired to the given store.
func New(s *store.Store) Model {
	items := linksToItems(s.All())
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 0, 0)
	l.Title = "🔥 hotclip [recent]"
	l.Styles.Title = titleStyle
	l.AdditionalShortHelpKeys = listKeys.ShortHelp
	l.AdditionalFullHelpKeys = func() []key.Binding { return listKeys.ShortHelp() }
	l.SetStatusBarItemName("link", "links")

	ti := textinput.New()
	ti.Placeholder = "New title"
	ti.CharLimit = 200
	ti.SetWidth(60)

	ui := textinput.New()
	ui.Placeholder = "https://"
	ui.CharLimit = 2000
	ui.SetWidth(60)

	return Model{
		store:      s,
		list:       l,
		titleInput: ti,
		urlInput:   ui,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		inputWidth := msg.Width - 8
		if inputWidth > 0 {
			m.titleInput.SetWidth(inputWidth)
			m.urlInput.SetWidth(inputWidth)
		}
		return m, nil
	}

	switch m.view {
	case viewList:
		return m.updateList(msg)
	case viewForm:
		return m.updateForm(msg)
	}
	return m, nil
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.MouseClickMsg); ok {
		if msg.Button == tea.MouseLeft {
			// Items start at Y=2 (title + status bar).
			// Each item is 1 line tall (no description, no spacing).
			const headerLines = 2
			clickedRow := msg.Y - headerLines
			pageOffset := m.list.Paginator.Page * m.list.Paginator.PerPage
			globalIdx := pageOffset + clickedRow
			visible := m.list.VisibleItems()
			if clickedRow >= 0 && globalIdx < len(visible) {
				m.list.Select(globalIdx)
				return m.copySelected()
			}
		}
		return m, nil
	}
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		// Handle delete confirmation.
		if m.confirmingDel {
			m.confirmingDel = false
			if msg.String() == "y" {
				return m.doDelete()
			}
			status := m.list.NewStatusMessage("Delete cancelled")
			return m, status
		}
		// Don't intercept single-letter keys while filtering.
		if !m.list.SettingFilter() {
			switch {
			case key.Matches(msg, listKeys.Copy):
				return m.copySelected()
			case key.Matches(msg, listKeys.Open):
				return m.openSelected()
			case key.Matches(msg, listKeys.Add):
				return m.enterForm()
			case key.Matches(msg, listKeys.Edit):
				return m.enterEditForm()
			case key.Matches(msg, listKeys.Delete):
				return m.deleteSelected()
			case key.Matches(msg, listKeys.Sort):
				sel, _ := m.list.SelectedItem().(linkItem)
				m.sortMode = m.sortMode.Next()
				m.refreshList()
				m.selectByID(sel.ID)
				status := m.list.NewStatusMessage(fmt.Sprintf("Sort: %s", m.sortMode.Label()))
				return m, status
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case titleFetchedMsg:
		if msg.title != "" && m.titleInput.Value() == "" {
			m.titleInput.SetValue(msg.title)
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.view = viewList
			return m, nil
		case "tab", "shift+tab":
			m.focusTitle = !m.focusTitle
			m = m.syncFormFocus()
			// When tabbing to title and it's empty, auto-fetch from URL.
			if m.focusTitle && m.titleInput.Value() == "" {
				url := strings.TrimSpace(m.urlInput.Value())
				if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
					return m, fetchPageTitle(url)
				}
			}
			return m, nil
		case "enter":
			return m.submitForm()
		}
	}

	var cmd tea.Cmd
	if m.focusTitle {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else {
		m.urlInput, cmd = m.urlInput.Update(msg)
	}
	return m, cmd
}

func (m Model) copySelected() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(linkItem)
	if !ok {
		return m, nil
	}
	_ = m.store.BumpActivity(item.ID)
	m.refreshList()
	m.selectByID(item.ID)
	title := item.Link.Title
	url := item.URL
	copyCmd := func() tea.Msg {
		if err := clipboard.CopyRichLink(title, url); err != nil {
			// Fallback to OSC52 plain text is not possible from inside a Cmd,
			// but on Windows the syscall approach should work reliably.
			_ = err
		}
		return nil
	}
	status := m.list.NewStatusMessage(fmt.Sprintf("Copied: %s", title))
	return m, tea.Batch(copyCmd, status)
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(linkItem)
	if !ok {
		return m, nil
	}
	_ = browser.OpenURL(item.URL)
	_ = m.store.BumpActivity(item.ID)
	m.refreshList()
	m.selectByID(item.ID)
	status := m.list.NewStatusMessage(fmt.Sprintf("Opened: %s", item.Link.Title))
	return m, status
}

func (m Model) deleteSelected() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(linkItem)
	if !ok {
		return m, nil
	}
	m.confirmingDel = true
	status := m.list.NewStatusMessage(fmt.Sprintf("Delete \"%s\"? y/n", item.Link.Title))
	return m, status
}

func (m Model) doDelete() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(linkItem)
	if !ok {
		return m, nil
	}
	_ = m.store.Delete(item.ID)
	m.refreshList()
	status := m.list.NewStatusMessage(fmt.Sprintf("Deleted: %s", item.Link.Title))
	return m, status
}

func (m Model) enterForm() (tea.Model, tea.Cmd) {
	m.view = viewForm
	m.editingID = ""
	m.focusTitle = false
	m.titleInput.SetValue("")
	m.urlInput.SetValue("")
	m = m.syncFormFocus()
	return m, m.urlInput.Focus()
}

func (m Model) enterEditForm() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(linkItem)
	if !ok {
		return m, nil
	}
	m.view = viewForm
	m.editingID = item.ID
	m.focusTitle = false
	m.titleInput.SetValue(item.Link.Title)
	m.urlInput.SetValue(item.URL)
	m = m.syncFormFocus()
	return m, m.urlInput.Focus()
}

func (m Model) syncFormFocus() Model {
	if m.focusTitle {
		m.urlInput.Blur()
		m.titleInput.Focus()
	} else {
		m.titleInput.Blur()
		m.urlInput.Focus()
	}
	return m
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	url := strings.TrimSpace(m.urlInput.Value())
	if url == "" {
		return m, nil
	}
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		title = url
	}

	var msg string
	if m.editingID != "" {
		_ = m.store.Update(m.editingID, title, url)
		msg = fmt.Sprintf("Updated: %s", title)
	} else {
		link := model.NewLink(title, url, nil)
		_ = m.store.Add(link)
		msg = fmt.Sprintf("Added: %s", title)
	}

	m.view = viewList
	m.editingID = ""
	m.refreshList()
	status := m.list.NewStatusMessage(msg)
	return m, status
}

func (m *Model) refreshList() {
	links := m.store.AllUnsorted()
	model.SortLinks(links, m.sortMode)
	m.list.SetItems(linksToItems(links))
	m.list.Title = fmt.Sprintf("🔥 hotclip [%s]", m.sortMode.Label())
}

// selectByID moves the cursor to the item with the given ID.
func (m *Model) selectByID(id string) {
	for i, item := range m.list.Items() {
		if li, ok := item.(linkItem); ok && li.ID == id {
			m.list.Select(i)
			return
		}
	}
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.view == viewForm {
		v.SetContent(m.formView())
		return v
	}

	if len(m.list.Items()) == 0 && !m.list.SettingFilter() {
		empty := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("🔥 hotclip"),
			"",
			dimStyle.Render("  No links yet. Press 'a' to add one."),
		)
		v.SetContent(lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, empty))
		return v
	}

	v.SetContent(m.list.View())
	return v
}

func linksToItems(links []model.Link) []list.Item {
	items := make([]list.Item, len(links))
	for i, l := range links {
		items[i] = linkItem{l}
	}
	return items
}
