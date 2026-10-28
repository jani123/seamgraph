package ui

import (
	"net/url"
	"seamgraph/internal/application"
	"strings"
)

func checklistMark(r application.ReviewWorkspace) application.ReviewMark {
	for _, mark := range r.Marks {
		if mark.Entry.Checklist {
			return mark
		}
	}
	return application.ReviewMark{Entry: application.ReviewEntry{Note: "- [ ] Read all changed files\n- [ ] Inspect affected consumers\n- [ ] Check tests and evidence\n- [ ] Resolve review questions"}}
}
func diffLayoutLink(browser application.CodeBrowser, filter CodeFilter, layout string) string {
	address, _ := url.Parse(codeLink(browser, filter, browser.Selected, browser.Object))
	q := address.Query()
	q.Set("layout", layout)
	address.RawQuery = q.Encode()
	return address.String()
}
func moveDescription(browser application.CodeBrowser) string {
	for _, f := range browser.Files {
		if f.Path == browser.Selected && f.OldPath != "" {
			return "Exact-content rename: " + f.OldPath + " → " + f.Path
		}
	}
	return "Exact-content moves are matched. Edited moves remain separate additions/deletions; their identity is uncertain."
}
func reviewFileGroup(r application.ReviewWorkspace, file string) string {

	for _, f := range r.Code.Files {
		if f.Path == file && f.Configuration {
			return "Root architecture configuration"
		}
		if f.Path == file && f.Module != "" && f.Module != "module:unassigned" {
			return f.Module
		}
	}
	return "Unassigned / root configuration"
}
func reviewFileSeams(r application.ReviewWorkspace, file string) string {
	seams := []string{}
	for _, o := range r.Code.Objects {
		if o.Symbol.Definition != nil && o.Symbol.Definition.Location.File == file {
			seams = append(seams, o.Seams...)
		}
	}
	return strings.Join(seams, ", ")
}

func evidenceLink(browser application.CodeBrowser, filter CodeFilter, side string) string {
	address, _ := url.Parse(codeLink(browser, filter, browser.Selected, browser.Object))
	q := address.Query()
	q.Set("evidence", side)
	address.RawQuery = q.Encode()
	return address.String()
}
