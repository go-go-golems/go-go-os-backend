package gepa

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ScriptDescriptor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type ScriptCatalog interface {
	List(ctx context.Context) ([]ScriptDescriptor, error)
}

type FileScriptCatalog struct {
	roots []string
}

func NewFileScriptCatalog(roots []string) *FileScriptCatalog {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return &FileScriptCatalog{roots: normalized}
}

func (c *FileScriptCatalog) List(context.Context) ([]ScriptDescriptor, error) {
	if c == nil || len(c.roots) == 0 {
		return []ScriptDescriptor{}, nil
	}

	scripts := make([]ScriptDescriptor, 0, 32)
	seen := map[string]struct{}{}

	for _, root := range c.roots {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(rootAbs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			name := d.Name()
			if d.IsDir() {
				switch name {
				case ".git", "node_modules", "dist", "build", "ttmp":
					return fs.SkipDir
				}
				return nil
			}
			if !isScriptFile(path) {
				return nil
			}
			rel, err := filepath.Rel(rootAbs, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			id := strings.TrimSpace(rel)
			if id == "" {
				return nil
			}
			if _, ok := seen[id]; ok {
				return nil
			}
			seen[id] = struct{}{}
			scripts = append(scripts, ScriptDescriptor{
				ID:   id,
				Name: strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)),
				Path: path,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(scripts, func(i, j int) bool { return scripts[i].ID < scripts[j].ID })
	return scripts, nil
}

func isScriptFile(path string) bool {
	lowered := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lowered, ".js") ||
		strings.HasSuffix(lowered, ".mjs") ||
		strings.HasSuffix(lowered, ".cjs")
}
