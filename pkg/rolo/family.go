package rolo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/lipgloss/tree"

	"github.com/asphaltbuffet/wherefolk/internal/tui"
)

type Directory struct {
	Root Family `json:"family"`
}

type Family struct {
	People    []Person `json:"people"`
	Marriage  string   `json:"marriage"`
	Children  []Family `json:"children"`
	Addresses []string `json:"addresses"`
}

func LoadJSON(fp string) ([]Family, error) {
	file, err := os.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	f := []Family{}

	err = json.Unmarshal(file, &f)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (f Family) rPrint(indent int) {
	pad := strings.Repeat("  ", indent)
	heads := []string{}

	for _, p := range f.People {
		heads = append(heads, p.getName())
	}

	fmt.Println(pad, strings.Join(heads, " & "))

	for _, c := range f.Children {
		c.rPrint(indent + 2)
	}
}

func (f Family) HasSharedName() bool {
	p := f.People
	return len(p) == 1 || p[0].Surname == p[1].Surname
}

func (f Family) familyName() string {
	var sb strings.Builder
	num := len(f.People)

	for i, p := range f.People {
		if i != 0 {
			sb.WriteString(" & ")
		}

		sb.WriteString(p.Name)

		if p.Aka != "" {
			sb.WriteString(fmt.Sprintf(" \"%s\"", p.Aka))
		}

		// add given/birth surname in parens
		if p.Given != "" && p.Surname != p.Given {
			sb.WriteString(fmt.Sprintf(" (%s)", p.Given))
		}

		if !f.HasSharedName() || num == 1 || i == 1 {
			sb.WriteString(" " + p.Surname)
		}
	}

	// return fmt.Sprint(sb.String())
	return tui.Household.Render(sb.String())
}

func (f Family) HasSubfamilies() bool {
	for _, c := range f.Children {
		if c.IsFamily() {
			return true
		}
	}

	return false
}

func (f Family) IsFamily() bool {
	return len(f.People) > 1 ||
		len(f.Children) > 0 ||
		len(f.Addresses) > 0
}

func (f Family) HasAddress() bool {
	return len(f.Addresses) != 0
}

func (f Family) Table(g int) string {
	var sb strings.Builder
	sb.WriteString(f.familyName())
	sb.WriteString("\n")

	// household address
	if f.HasAddress() {
		sb.WriteString("\n")
		for _, a := range f.Addresses {
			sb.WriteString(tui.Indent.Render(tui.Address.Render(a)))
			sb.WriteString("\n")
		}
	}

	// heads of household: Name | Birth | Phone | Email
	hohTable := table.New().Border(lipgloss.HiddenBorder())
	for _, p := range f.People {
		hohTable.Row(p.Name, p.Birth, p.Phone, p.Email)
	}
	sb.WriteString(tui.Indent.Render(fmt.Sprint(hohTable)))
	sb.WriteString("\n")

	if f.Marriage != "" {
		sb.WriteString(tui.Indent.Render(tui.Anniversary.Render(f.Marriage)))
		sb.WriteString("\n")
	}

	// dependents: Name | Birth [~~ Death] | Phone | Email
	var dependents []Family
	for _, c := range f.Children {
		if !c.IsFamily() {
			dependents = append(dependents, c)
		}
	}
	if len(dependents) > 0 {
		sb.WriteString("\n")
		depTable := table.New().Border(lipgloss.HiddenBorder())
		for _, c := range dependents {
			p := c.People[0]
			birthDeath := p.Birth
			if p.Death != "" {
				birthDeath = p.Birth + " ~~ " + p.Death
			}
			depTable.Row(p.Name, birthDeath, p.Phone, p.Email)
		}
		sb.WriteString(tui.Indent.Render(fmt.Sprint(depTable)))
		sb.WriteString("\n")
	}

	for _, c := range f.Children {
		if c.IsFamily() {
			sb.WriteString("\n")
			sb.WriteString(c.Table(0))
		}
	}

	return sb.String()
}

func (f Family) Info(g int) string {
	var sb strings.Builder
	sb.WriteString(f.familyName())

	// household address
	if f.HasAddress() {
		for _, a := range f.Addresses {
			sb.WriteString("\n")
			sb.WriteString(tui.Address.Render(a))
		}

	}

	// details for heads of household
	for _, p := range f.People {
		sb.WriteString("\n")
		sb.WriteString(p.Details()[0])
	}

	if f.Marriage != "" {
		sb.WriteString("\n")
		sb.WriteString(tui.Anniversary.Render(f.Marriage))
	}

	// details for kids still at home (or deceased)
	for _, c := range f.Children {
		if !c.IsFamily() {
			sb.WriteString("\n")
			sb.WriteString(c.People[0].Details()[0])
		}
	}

	for _, c := range f.Children {
		if c.IsFamily() {
			sb.WriteString("\n")
			sb.WriteString(c.Info(0))
		}
	}

	return sb.String()
}

func (f Family) MakeTree() *tree.Tree {
	name := f.familyName()
	vbar := " "

	if f.HasSubfamilies() {
		vbar = "│"
	}

	// household address
	if len(f.Addresses) != 0 {
		name += fmt.Sprintf("\n%s", vbar)

		for _, a := range f.Addresses {
			name += fmt.Sprintf("\n%s  %s", vbar, tui.Address.Render(a))
		}

		name += fmt.Sprintf("\n%s", vbar)
	}

	// details for heads of household
	for _, p := range f.People {
		name += fmt.Sprintf("\n%s  %s", vbar, p.Details())
	}

	if f.Marriage != "" {
		m := tui.Anniversary.Render(f.Marriage)
		name += fmt.Sprintf("\n%s  %s", vbar, m)

		name += fmt.Sprintf("\n%s", vbar)
	}

	// details for kids still at home (or deceased)
	var extra bool
	for _, c := range f.Children {
		if !c.IsFamily() {
			extra = true
			name += fmt.Sprintf("\n%s  %s", vbar, c.People[0].Details())
		}
	}

	if extra {
		name += fmt.Sprintf("\n%s", vbar)
	}

	t := tree.New().
		Root(name)

	for _, c := range f.Children {
		if c.IsFamily() {
			t.Child(c.MakeTree())
		}
	}

	return t
}
