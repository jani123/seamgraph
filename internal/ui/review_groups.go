package ui

import (
	"seamgraph/internal/application"
	"slices"
	"strings"
)

type reviewGroup struct {
	Name  string
	Files []application.ReviewFile
}

func reviewGroups(r application.ReviewWorkspace) []reviewGroup {
	groups := map[string][]application.ReviewFile{}
	for _, file := range r.Code.Files {
		if file.Status != "unchanged" {
			name := reviewFileGroup(r, file.Path)
			if seams := reviewFileSeams(r, file.Path); seams != "" {
				name += " · seams: " + seams
			}
			groups[name] = append(groups[name], file)
		}
	}
	result := []reviewGroup{}
	for name, files := range groups {
		result = append(result, reviewGroup{name, files})
	}
	slices.SortFunc(result, func(a, b reviewGroup) int { return strings.Compare(a.Name, b.Name) })
	return result
}
