package ui

import (
	"fmt"
	"strconv"
	"strings"

	"seamgraph/internal/application"
)

// Source ports return numbered text; keep numbering out of the copied source
// and syntax tokenizer, and render a separate, nonselectable gutter instead.
func sourcePlain(numbered string) string {
	lines := strings.Split(strings.TrimSuffix(numbered, "\n"), "\n")
	for i, line := range lines {
		number, text, ok := strings.Cut(strings.TrimLeft(line, " "), "  ")
		if _, err := strconv.Atoi(number); ok && err == nil {
			lines[i] = text
		}
	}
	return strings.Join(lines, "\n")
}
func sourceNumbers(numbered string) string {
	if numbered == "" {
		return ""
	}
	var numbers strings.Builder
	count := strings.Count(sourcePlain(numbered), "\n") + 1
	for i := 1; i <= count; i++ {
		fmt.Fprintln(&numbers, i)
	}
	return numbers.String()
}
func codeFocus(browser application.CodeBrowser) int {
	object := selectedCode(browser)
	if object == nil || object.Symbol.Definition == nil {
		return 0
	}
	return object.Symbol.Definition.Location.StartLine
}
