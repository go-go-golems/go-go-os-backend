package docmw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestParseFS_SuccessAndOrdering(t *testing.T) {
	fsys := fstest.MapFS{
		"01-overview.md": {Data: []byte(`---
Title: Overview
DocType: guide
Topics: [backend, onboarding]
Order: 2
---
hello`)},
		"00-api.md": {Data: []byte(`---
Title: API
DocType: reference
Topics:
  - api
Order: 1
---
api docs`)},
	}

	store, err := ParseFS("inventory", fsys, ParseOptions{})
	require.NoError(t, err)
	require.Equal(t, "inventory", store.ModuleID)
	require.Len(t, store.Docs, 2)
	require.Equal(t, "00-api", store.Docs[0].Slug)
	require.Equal(t, "01-overview", store.Docs[1].Slug)
	require.Equal(t, 2, store.Count())
}

func TestParseFS_MissingRequiredFrontmatterFails(t *testing.T) {
	fsys := fstest.MapFS{
		"overview.md": {Data: []byte(`---
DocType: guide
---
hello`)},
	}

	_, err := ParseFS("inventory", fsys, ParseOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required frontmatter field: title")
}

func TestNewDocStore_DuplicateSlugFails(t *testing.T) {
	_, err := NewDocStore("inventory", []ModuleDoc{
		{Slug: "overview", Title: "Overview", DocType: "guide"},
		{Slug: "overview", Title: "Overview 2", DocType: "guide"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate doc slug")
}

func TestMountRoutes_TOCAndDocEndpoints(t *testing.T) {
	store, err := NewDocStore("inventory", []ModuleDoc{
		{
			Slug:    "overview",
			Title:   "Overview",
			DocType: "guide",
			Topics:  []string{"backend"},
			Content: "hello",
		},
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	require.NoError(t, MountRoutes(mux, store))

	tocReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	tocRes := httptest.NewRecorder()
	mux.ServeHTTP(tocRes, tocReq)
	require.Equal(t, http.StatusOK, tocRes.Code)

	var tocPayload struct {
		ModuleID string      `json:"module_id"`
		Docs     []ModuleDoc `json:"docs"`
	}
	require.NoError(t, json.NewDecoder(tocRes.Body).Decode(&tocPayload))
	require.Equal(t, "inventory", tocPayload.ModuleID)
	require.Len(t, tocPayload.Docs, 1)
	require.Equal(t, "", tocPayload.Docs[0].Content, "toc should exclude content")

	docReq := httptest.NewRequest(http.MethodGet, "/docs/overview", nil)
	docRes := httptest.NewRecorder()
	mux.ServeHTTP(docRes, docReq)
	require.Equal(t, http.StatusOK, docRes.Code)

	var docPayload ModuleDoc
	require.NoError(t, json.NewDecoder(docRes.Body).Decode(&docPayload))
	require.Equal(t, "overview", docPayload.Slug)
	require.Equal(t, "hello", docPayload.Content)

	notFoundReq := httptest.NewRequest(http.MethodGet, "/docs/missing", nil)
	notFoundRes := httptest.NewRecorder()
	mux.ServeHTTP(notFoundRes, notFoundReq)
	require.Equal(t, http.StatusNotFound, notFoundRes.Code)
}

func TestParseFS_StrictVocabularyFailsOnUnknownValues(t *testing.T) {
	fsys := fstest.MapFS{
		"vocabulary.yaml": {Data: []byte(`topics: [backend]
doc_types: [guide]
`)},
		"overview.md": {Data: []byte(`---
Title: Overview
DocType: guide
Topics: [unknown-topic]
---
hello`)},
	}

	v, err := LoadVocabularyFS(fsys, "vocabulary.yaml")
	require.NoError(t, err)

	_, err = ParseFS("inventory", fsys, ParseOptions{
		Vocabulary:       v,
		StrictVocabulary: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown topic")
}
