package ui

import (
	"net/url"
	"seamgraph/internal/application"
)

func commentExportLink(browser application.CodeBrowser) string {
	return "/explore?" + url.Values{"base": {browser.ReviewBaseRef}, "revision": {browser.ReviewHeadRef}, "export": {"comments"}}.Encode()
}
func commentSideLabel(side string) string {
	if side == "before" {
		return "Base"
	}
	return "Head"
}

func changeComments(browser application.CodeBrowser) []application.ReviewMark {
	result := []application.ReviewMark{}
	for _, mark := range browser.Marks {
		if mark.Entry.Comment {
			result = append(result, mark)
		}
	}
	return result
}

func commentVersion(entry application.ReviewEntry) string {
	if entry.UpdatedAt != "" {
		return entry.UpdatedAt
	}
	return entry.RecordedAt
}
