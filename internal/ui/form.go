package ui

import (
	"strings"
)

func (m Model) formView() string {
	var b strings.Builder

	heading := "  Add New Link"
	if m.editingID != "" {
		heading = "  Edit Link"
	}
	b.WriteString("\n")
	b.WriteString(titleStyle.Render(heading))
	b.WriteString("\n\n")
	b.WriteString(formLabelStyle.Render("URL"))
	b.WriteString("\n")
	b.WriteString(formFieldStyle.Render(m.urlInput.View()))
	b.WriteString("\n\n")
	b.WriteString(formLabelStyle.Render("Title"))
	b.WriteString("\n")
	b.WriteString(formFieldStyle.Render(m.titleInput.View()))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.PaddingLeft(2).Render("enter submit • tab next field • esc cancel"))

	return b.String()
}
