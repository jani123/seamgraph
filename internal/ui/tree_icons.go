package ui

import "seamgraph/internal/application"

var treeKindPaths = map[string]string{
	"function":    "M6 17 9 5q1-3 5-2M5 8h8m0 3 5 5m0-5-5 5",
	"method":      "M3 3h14v14H3zM8 7l-3 3 3 3m4-6 3 3-3 3",
	"class":       "M10 2l7 4v8l-7 4-7-4V6zM3 6l7 4 7-4m-7 4v8",
	"struct":      "M3 3h14v14H3zM3 8h14M8 8v9",
	"interface":   "M6 5a7 7 0 1 0 8 0M10 2v8M7 2h6",
	"type":        "M10 2l8 8-8 8-8-8zM7 7h6m-3 0v6",
	"enum":        "M7 4h10M7 10h10M7 16h10M3 4h.1M3 10h.1M3 16h.1",
	"field":       "M3 6h14v8H3zM7 6v8",
	"property":    "M10 3l7 7-7 7-7-7zM7 10h6",
	"variable":    "M6 3H4v14h2m8-14h2v14h-2M8 7l4 6m0-6-4 6",
	"constant":    "M10 2l7 8-7 8-7-8zM7 8h6m-6 4h6",
	"constructor": "M3 3h14v14H3zM10 6v8m-4-4h8",
	"alias":       "M3 10h14m-5-5 5 5-5 5",
	"namespace":   "M2 5h6l2 2h8v10H2z",
}
var treeKindLabels = map[string]string{
	"function": "Function", "method": "Method", "class": "Class", "struct": "Struct", "interface": "Interface", "type": "Type", "enum": "Enum", "field": "Field", "property": "Property", "variable": "Variable", "constant": "Constant", "constructor": "Constructor", "alias": "Type alias", "type-alias": "Type alias", "namespace": "Namespace",
}
var treeStatusPaths = map[string]string{
	"added":    "M10 3v14M3 10h14",
	"removed":  "M3 10h14",
	"modified": "M4 13l9-9 3 3-9 9-4 1zM11 6l3 3",
	"renamed":  "M3 10h14m-5-5 5 5-5 5",
}
var treeStatusLabels = map[string]string{"added": "Added", "removed": "Removed", "modified": "Modified", "renamed": "Renamed", "unchanged": "Unchanged"}

func treeKindLabel(kind string) string {
	if label := treeKindLabels[kind]; label != "" {
		return label
	}
	if kind == "" {
		return "Symbol"
	}
	return kind
}
func treeKindPath(kind string) string {
	if path := treeKindPaths[kind]; path != "" {
		return path
	}
	return treeKindPaths["type"]
}
func treeStatusLabel(status string) string {
	if label := treeStatusLabels[status]; label != "" {
		return label
	}
	return status
}
func treeObjectDescription(object application.CodeObject) string {
	return treeKindLabel(object.Symbol.Kind) + " · " + treeStatusLabel(object.Status)
}
func treeFileDescription(status string) string { return "File · " + treeStatusLabel(status) }
func treeEntryKind(directory bool) string {
	if directory {
		return "Directory"
	}
	return "File"
}
