package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHexagonalLayerImports(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	root := filepath.Join("..", "modules")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		slash := filepath.ToSlash(path)
		module := moduleName(slash)
		layer := detectLayer(slash)
		if module == "" || layer == "" {
			return nil
		}
		node, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if !strings.Contains(importPath, "mdht/internal/modules/") {
				continue
			}
			if violatesLayerRule(module, layer, importPath) {
				t.Fatalf("forbidden import in %s (%s): %s", slash, layer, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

func moduleName(path string) string {
	parts := strings.Split(path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "modules" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func detectLayer(path string) string {
	for _, layer := range []string{"adapter/in", "adapter/out", "usecase", "service", "domain", "port/in", "port/out", "dto"} {
		if strings.Contains(path, "/"+layer+"/") {
			return layer
		}
	}
	return ""
}

func isPortIn(path string) bool {
	return strings.Contains(path, "/port/in/") || strings.HasSuffix(path, "/port/in")
}

func isDTO(path string) bool {
	return strings.Contains(path, "/dto/") || strings.HasSuffix(path, "/dto")
}

func violatesLayerRule(module, layer, importPath string) bool {
	sameModule := strings.Contains(importPath, "/internal/modules/"+module+"/")
	if !sameModule {
		if strings.Contains(importPath, "/service/") || strings.Contains(importPath, "/adapter/") || strings.Contains(importPath, "/usecase/") {
			return true
		}
		if isPortIn(importPath) || isDTO(importPath) {
			return false
		}
	}

	switch layer {
	case "adapter/in":
		return !isPortIn(importPath) && !isDTO(importPath)
	case "usecase":
		return strings.Contains(importPath, "/adapter/")
	case "service":
		return strings.Contains(importPath, "/adapter/") || strings.Contains(importPath, "/usecase/")
	case "domain":
		return strings.Contains(importPath, "/adapter/") || strings.Contains(importPath, "/usecase/") || strings.Contains(importPath, "/service/")
	default:
		return false
	}
}
