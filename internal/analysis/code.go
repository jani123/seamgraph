package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

// Index analyzes all supported production sources using one synthetic owner per
// adapter. It never creates configuration or invents architecture seams.
func (s Scanner) Index(ctx context.Context, root string, files map[string]string) (ir.CodeIndex, error) {
	result := ir.CodeIndex{Issues: []string{}, DeclaredModules: []ir.Module{}, DeclaredSeams: []ir.Seam{}}
	doc, err := s.Load(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		result.Issues = append(result.Issues, "Architecture configuration unavailable: "+err.Error())
	}
	languages := codeLanguages(files)
	parts := []ir.PartialSnapshot{}
	for _, language := range languages {
		if s.Adapters[language] == nil {
			result.Issues = append(result.Issues, fmt.Sprintf("%s adapter unavailable", language))
			continue
		}
		module := config.Module{Language: language, Paths: []string{"."}, ImportRoots: codeImportRoots(files, doc)}
		if language == "typescript" {
			module.TSConfig, err = codeTSConfig(files, doc)
			if err != nil {
				result.Issues = append(result.Issues, err.Error())
				continue
			}
		}
		cfg := config.Config{Version: 1, Modules: map[string]config.Module{"unassigned": module}, Policies: config.Policies{Default: "allow", Unresolved: "warn"}}
		part, err := s.Adapters[language].Analyze(ctx, Project{Root: root}, cfg)
		if err != nil {
			result.Issues = append(result.Issues, language+": "+err.Error())
			continue
		}
		parts = append(parts, part)
	}
	result.Facts, err = ir.Merge(parts...)
	if err != nil {
		return result, err
	}
	if doc != nil {
		result.DeclaredModules, result.DeclaredSeams = doc.Config.IRModules()
		declared, _, err := s.Scan(ctx, root)
		if err != nil {
			result.Issues = append(result.Issues, "Seam membership unavailable: "+err.Error())
		} else {
			result.DeclaredSeams = declared.Seams
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}
func codeLanguages(files map[string]string) []string {
	languages := []string{}
	for file := range files {
		language := ""
		switch path.Ext(file) {
		case ".go":
			language = "go"
		case ".ts", ".tsx", ".mts", ".cts":
			language = "typescript"
		case ".py":
			language = "python"
		}
		if language != "" && !slices.Contains(languages, language) {
			languages = append(languages, language)
		}
	}
	slices.Sort(languages)
	return languages
}
func codeImportRoots(files map[string]string, doc *config.Document) []string {
	roots := []string{"."}
	if doc != nil {
		for _, module := range doc.Config.Modules {
			roots = append(roots, module.ImportRoots...)
		}
	}
	for file := range files {
		if strings.HasSuffix(file, "/__init__.py") {
			roots = append(roots, path.Dir(path.Dir(file)))
		}
	}
	slices.Sort(roots)
	return slices.Compact(roots)
}

// Match the adapter's one shared TypeScript project rather than silently using
// default compiler options when declared roots discover a nested tsconfig.
func codeTSConfig(files map[string]string, doc *config.Document) (string, error) {
	configs := []string{}
	if doc != nil {
		for _, module := range doc.Config.Modules {
			if module.Language != "typescript" {
				continue
			}
			if module.TSConfig != "" {
				configs = append(configs, module.TSConfig)
				continue
			}
			for _, root := range module.Paths {
				for current := root; ; current = path.Dir(current) {
					candidate := path.Join(current, "tsconfig.json")
					if _, ok := files[candidate]; ok {
						configs = append(configs, candidate)
						break
					}
					if current == "." {
						break
					}
				}
			}
		}
	}
	slices.Sort(configs)
	configs = slices.Compact(configs)
	if len(configs) > 1 {
		return "", fmt.Errorf("typescript: multiple independent tsconfig projects are unsupported")
	}
	if len(configs) == 1 {
		return configs[0], nil
	}
	return "", nil
}
