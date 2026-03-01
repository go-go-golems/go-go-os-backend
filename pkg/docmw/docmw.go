package docmw

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// ModuleDoc represents one documentation page exposed by a backend module.
type ModuleDoc struct {
	ModuleID string   `json:"module_id"`
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	DocType  string   `json:"doc_type"`
	Topics   []string `json:"topics,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	SeeAlso  []string `json:"see_also,omitempty"`
	Order    int      `json:"order,omitempty"`
	Content  string   `json:"content,omitempty"`
}

type ParseOptions struct {
	Vocabulary       *Vocabulary
	StrictVocabulary bool
}

type DocStore struct {
	ModuleID string
	Docs     []ModuleDoc
	BySlug   map[string]*ModuleDoc
}

type tocResponse struct {
	ModuleID string      `json:"module_id"`
	Docs     []ModuleDoc `json:"docs"`
}

func ParseFS(moduleID string, fsys fs.FS, opts ParseOptions) (*DocStore, error) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return nil, errors.New("module id is required")
	}
	if fsys == nil {
		return nil, errors.New("fs is required")
	}

	files := make([]string, 0, 16)
	err := fs.WalkDir(fsys, ".", func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(filePath), ".md") {
			return nil
		}
		files = append(files, filePath)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "walk docs fs")
	}

	docs := make([]ModuleDoc, 0, len(files))
	for _, filePath := range files {
		raw, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return nil, errors.Wrapf(err, "read %q", filePath)
		}
		doc, err := parseMarkdown(moduleID, filePath, string(raw), opts)
		if err != nil {
			return nil, errors.Wrapf(err, "parse %q", filePath)
		}
		docs = append(docs, doc)
	}

	return NewDocStore(moduleID, docs)
}

func NewDocStore(moduleID string, docs []ModuleDoc) (*DocStore, error) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return nil, errors.New("module id is required")
	}

	out := make([]ModuleDoc, 0, len(docs))
	for _, doc := range docs {
		doc.ModuleID = moduleID
		doc.Slug = normalizeSlug(doc.Slug)
		doc.Title = strings.TrimSpace(doc.Title)
		doc.DocType = strings.TrimSpace(doc.DocType)
		doc.Summary = strings.TrimSpace(doc.Summary)
		doc.Topics = normalizeList(doc.Topics)
		doc.SeeAlso = normalizeList(doc.SeeAlso)

		if doc.Slug == "" {
			return nil, errors.New("doc slug is required")
		}
		if doc.Title == "" {
			return nil, errors.Errorf("doc %q missing title", doc.Slug)
		}
		if doc.DocType == "" {
			return nil, errors.Errorf("doc %q missing doc_type", doc.Slug)
		}
		out = append(out, doc)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Slug < out[j].Slug
	})

	bySlug := make(map[string]*ModuleDoc, len(out))
	for i := range out {
		slug := out[i].Slug
		if _, ok := bySlug[slug]; ok {
			return nil, errors.Errorf("duplicate doc slug %q", slug)
		}
		bySlug[slug] = &out[i]
	}

	return &DocStore{
		ModuleID: moduleID,
		Docs:     out,
		BySlug:   bySlug,
	}, nil
}

func (s *DocStore) Count() int {
	if s == nil {
		return 0
	}
	return len(s.Docs)
}

func (s *DocStore) TOC() []ModuleDoc {
	if s == nil || len(s.Docs) == 0 {
		return nil
	}
	out := make([]ModuleDoc, 0, len(s.Docs))
	for _, doc := range s.Docs {
		copyDoc := doc
		copyDoc.Content = ""
		out = append(out, copyDoc)
	}
	return out
}

func (s *DocStore) Get(slug string) (*ModuleDoc, bool) {
	if s == nil {
		return nil, false
	}
	doc, ok := s.BySlug[normalizeSlug(slug)]
	if !ok || doc == nil {
		return nil, false
	}
	copyDoc := *doc
	return &copyDoc, true
}

// MountRoutes mounts /docs and /docs/{slug} handlers on a module-local mux.
func MountRoutes(mux *http.ServeMux, store *DocStore) error {
	if mux == nil {
		return errors.New("mux is required")
	}
	if store == nil {
		return errors.New("doc store is required")
	}

	mux.HandleFunc("/docs", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, tocResponse{
			ModuleID: store.ModuleID,
			Docs:     store.TOC(),
		})
	})

	mux.HandleFunc("/docs/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		slug := strings.TrimPrefix(req.URL.Path, "/docs/")
		slug = strings.TrimSpace(slug)
		if slug == "" || strings.Contains(slug, "/") {
			http.NotFound(w, req)
			return
		}
		doc, ok := store.Get(slug)
		if !ok {
			http.NotFound(w, req)
			return
		}
		writeJSON(w, doc)
	})

	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func parseMarkdown(moduleID, filePath, raw string, opts ParseOptions) (ModuleDoc, error) {
	var frontmatter map[string]any
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		var fmText string
		fmText, body = splitFrontmatter(raw)
		if strings.TrimSpace(fmText) == "" {
			return ModuleDoc{}, errors.New("empty frontmatter")
		}
		if err := yaml.Unmarshal([]byte(fmText), &frontmatter); err != nil {
			return ModuleDoc{}, errors.Wrap(err, "decode frontmatter yaml")
		}
	} else {
		frontmatter = map[string]any{}
	}

	title := metadataString(frontmatter, "title")
	docType := metadataString(frontmatter, "doctype", "doc_type")
	slug := metadataString(frontmatter, "slug")
	if strings.TrimSpace(slug) == "" {
		base := strings.TrimSuffix(path.Base(filePath), path.Ext(filePath))
		slug = base
	}

	doc := ModuleDoc{
		ModuleID: moduleID,
		Slug:     slug,
		Title:    title,
		DocType:  docType,
		Topics:   metadataStringSlice(frontmatter, "topics"),
		Summary:  metadataString(frontmatter, "summary"),
		SeeAlso:  metadataStringSlice(frontmatter, "seealso", "see_also"),
		Order:    metadataInt(frontmatter, "order"),
		Content:  strings.TrimSpace(body),
	}

	if strings.TrimSpace(doc.Title) == "" {
		return ModuleDoc{}, errors.New("missing required frontmatter field: title")
	}
	if strings.TrimSpace(doc.DocType) == "" {
		return ModuleDoc{}, errors.New("missing required frontmatter field: doc_type")
	}

	if opts.Vocabulary != nil {
		if err := opts.Vocabulary.ValidateDoc(doc, opts.StrictVocabulary); err != nil {
			return ModuleDoc{}, err
		}
	}

	return doc, nil
}

func splitFrontmatter(raw string) (string, string) {
	start := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(start, "\n---\n")
	if idx < 0 {
		return "", raw
	}
	fm := start[:idx]
	body := start[idx+len("\n---\n"):]
	return fm, body
}

func normalizeSlug(slug string) string {
	slug = strings.TrimSpace(strings.ToLower(slug))
	slug = strings.ReplaceAll(slug, "_", "-")
	return slug
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func metadataString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		for existingKey, rawValue := range m {
			if strings.EqualFold(existingKey, key) {
				if s, ok := rawValue.(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return ""
}

func metadataInt(m map[string]any, keys ...string) int {
	for _, key := range keys {
		for existingKey, rawValue := range m {
			if !strings.EqualFold(existingKey, key) {
				continue
			}
			switch v := rawValue.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			case string:
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func metadataStringSlice(m map[string]any, keys ...string) []string {
	for _, key := range keys {
		for existingKey, rawValue := range m {
			if !strings.EqualFold(existingKey, key) {
				continue
			}
			switch v := rawValue.(type) {
			case []any:
				out := make([]string, 0, len(v))
				for _, item := range v {
					if s, ok := item.(string); ok {
						out = append(out, s)
					}
				}
				return out
			case []string:
				return append([]string(nil), v...)
			case string:
				if strings.TrimSpace(v) == "" {
					return nil
				}
				return []string{v}
			}
		}
	}
	return nil
}
