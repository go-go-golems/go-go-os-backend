package vmmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	jsdocbatch "github.com/go-go-golems/go-go-goja/pkg/jsdoc/batch"
	jsdocmodel "github.com/go-go-golems/go-go-goja/pkg/jsdoc/model"
	"github.com/go-go-golems/go-go-goja/pkg/jsparse"
	"github.com/pkg/errors"
)

type GenerateOptions struct {
	PackID     string
	CardsDir   string
	DocsDir    string
	OutputJSON string
	OutputTS   string
}

type GeneratedPack struct {
	PackID string               `json:"packId"`
	Cards  []CardMetadata       `json:"cards"`
	Docs   *jsdocmodel.DocStore `json:"docs"`
}

type CardMetadata struct {
	ID           string   `json:"id"`
	PackID       string   `json:"packId"`
	Title        string   `json:"title"`
	Icon         string   `json:"icon"`
	SourceFile   string   `json:"sourceFile"`
	Source       string   `json:"source"`
	HandlerNames []string `json:"handlerNames"`
}

type cardSentinel struct {
	ID     string `json:"id"`
	PackID string `json:"packId"`
	Title  string `json:"title"`
	Icon   string `json:"icon"`
}

var trailingCommaPattern = regexp.MustCompile(`,(\s*[}\]])`)

func GenerateAndWrite(ctx context.Context, opts GenerateOptions) error {
	output, err := Generate(ctx, opts)
	if err != nil {
		return err
	}

	jsonBytes, err := renderJSON(output)
	if err != nil {
		return err
	}
	tsBytes, err := renderTypeScript(output)
	if err != nil {
		return err
	}

	if err := writeFile(opts.OutputJSON, jsonBytes); err != nil {
		return err
	}
	if err := writeFile(opts.OutputTS, tsBytes); err != nil {
		return err
	}

	return nil
}

func Generate(ctx context.Context, opts GenerateOptions) (*GeneratedPack, error) {
	if strings.TrimSpace(opts.PackID) == "" {
		return nil, errors.New("pack id is required")
	}
	if strings.TrimSpace(opts.CardsDir) == "" {
		return nil, errors.New("cards dir is required")
	}

	cardFiles, err := listJSFiles(opts.CardsDir)
	if err != nil {
		return nil, err
	}
	if len(cardFiles) == 0 {
		return nil, errors.Errorf("no VM card files found in %s", opts.CardsDir)
	}

	parser, err := jsparse.NewTSParser()
	if err != nil {
		return nil, errors.Wrap(err, "creating tree-sitter parser")
	}
	defer parser.Close()

	cards := make([]CardMetadata, 0, len(cardFiles))
	for _, path := range cardFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		card, err := parseCardFile(parser, path, opts.PackID)
		if err != nil {
			return nil, err
		}
		cards = append(cards, *card)
	}

	slices.SortFunc(cards, func(a, b CardMetadata) int {
		return strings.Compare(a.ID, b.ID)
	})

	docs, err := buildDocsStore(ctx, opts.DocsDir, cardFiles)
	if err != nil {
		return nil, err
	}

	return &GeneratedPack{
		PackID: opts.PackID,
		Cards:  cards,
		Docs:   docs,
	}, nil
}

func buildDocsStore(ctx context.Context, docsDir string, cardFiles []string) (*jsdocmodel.DocStore, error) {
	inputs := make([]jsdocbatch.InputFile, 0, len(cardFiles)+4)

	if strings.TrimSpace(docsDir) != "" {
		docFiles, err := listJSFiles(docsDir)
		if err != nil {
			return nil, err
		}
		for _, path := range docFiles {
			inputs = append(inputs, jsdocbatch.InputFile{Path: path})
		}
	}

	for _, path := range cardFiles {
		inputs = append(inputs, jsdocbatch.InputFile{Path: path})
	}

	result, err := jsdocbatch.BuildStore(ctx, inputs, jsdocbatch.BatchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "building docs store")
	}
	return result.Store, nil
}

func parseCardFile(parser *jsparse.TSParser, path string, expectedPackID string) (*CardMetadata, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", path)
	}

	root := parser.Parse(source)
	if root == nil {
		return nil, errors.Errorf("parsing %s: got nil tree", path)
	}

	slicer := newSourceSlicer(source)
	cardCall := findCallByName(root, slicer, "__card__")
	if cardCall == nil {
		return nil, errors.Errorf("%s: missing __card__(...) sentinel", path)
	}

	cardObject := firstDescendantOfKind(cardCall, "object")
	if cardObject == nil {
		return nil, errors.Errorf("%s: __card__(...) must include an object literal", path)
	}

	var sentinel cardSentinel
	if err := parseObjectNode(cardObject, slicer, &sentinel); err != nil {
		return nil, errors.Wrapf(err, "parsing __card__ metadata in %s", path)
	}
	if sentinel.ID == "" {
		return nil, errors.Errorf("%s: __card__ metadata is missing id", path)
	}
	if sentinel.PackID == "" {
		return nil, errors.Errorf("%s: __card__ metadata is missing packId", path)
	}
	if expectedPackID != "" && sentinel.PackID != expectedPackID {
		return nil, errors.Errorf("%s: __card__ packId %q does not match expected %q", path, sentinel.PackID, expectedPackID)
	}

	defineCardCall := findCallByName(root, slicer, "defineCard")
	if defineCardCall == nil {
		return nil, errors.Errorf("%s: missing defineCard(...) call", path)
	}

	handlersObject := findNamedObject(defineCardCall, slicer, "handlers")
	handlerNames := collectObjectKeys(handlersObject, slicer)
	slices.Sort(handlerNames)

	relativePath := path
	if rel, err := filepath.Rel(".", path); err == nil {
		relativePath = filepath.ToSlash(rel)
	}

	return &CardMetadata{
		ID:           sentinel.ID,
		PackID:       sentinel.PackID,
		Title:        sentinel.Title,
		Icon:         sentinel.Icon,
		SourceFile:   relativePath,
		Source:       string(source),
		HandlerNames: handlerNames,
	}, nil
}

func listJSFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory %s", dir)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	slices.Sort(files)
	return files, nil
}

func renderJSON(output *GeneratedPack) ([]byte, error) {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "marshalling JSON output")
	}
	return append(data, '\n'), nil
}

func renderTypeScript(output *GeneratedPack) ([]byte, error) {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "marshalling TypeScript payload")
	}

	content := strings.Join([]string{
		"// Code generated by go-go-os-backend vmmeta generate; DO NOT EDIT.",
		"",
		"export const VM_PACK_METADATA = " + string(data) + " as const;",
		"",
		"export default VM_PACK_METADATA;",
		"",
	}, "\n")

	return []byte(content), nil
}

func writeFile(path string, content []byte) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("output path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errors.Wrapf(err, "creating directory for %s", path)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return errors.Wrapf(err, "writing %s", path)
	}
	return nil
}

type sourceSlicer struct {
	source     []byte
	lineStarts []int
}

func newSourceSlicer(source []byte) sourceSlicer {
	lineStarts := []int{0}
	for idx, b := range source {
		if b == '\n' {
			lineStarts = append(lineStarts, idx+1)
		}
	}
	return sourceSlicer{source: source, lineStarts: lineStarts}
}

func (s sourceSlicer) text(node *jsparse.TSNode) string {
	if node == nil {
		return ""
	}
	start, ok := s.offset(node.StartRow, node.StartCol)
	if !ok {
		return ""
	}
	end, ok := s.offset(node.EndRow, node.EndCol)
	if !ok || end < start || end > len(s.source) {
		return ""
	}
	return string(s.source[start:end])
}

func (s sourceSlicer) offset(row, col int) (int, bool) {
	if row < 0 || row >= len(s.lineStarts) {
		return 0, false
	}
	start := s.lineStarts[row] + col
	if start < 0 || start > len(s.source) {
		return 0, false
	}
	return start, true
}

func findCallByName(root *jsparse.TSNode, slicer sourceSlicer, name string) *jsparse.TSNode {
	var found *jsparse.TSNode
	walk(root, func(node *jsparse.TSNode) bool {
		if node.Kind != "call_expression" {
			return false
		}
		if callName(node, slicer) == name {
			found = node
			return true
		}
		return false
	})
	return found
}

func callName(node *jsparse.TSNode, slicer sourceSlicer) string {
	for idx, child := range node.Children {
		if child.Kind != "arguments" {
			continue
		}
		for prev := idx - 1; prev >= 0; prev-- {
			if isTrivia(node.Children[prev]) {
				continue
			}
			return strings.TrimSpace(slicer.text(node.Children[prev]))
		}
	}
	return ""
}

func findNamedObject(root *jsparse.TSNode, slicer sourceSlicer, name string) *jsparse.TSNode {
	var found *jsparse.TSNode
	walk(root, func(node *jsparse.TSNode) bool {
		if node.Kind != "pair" {
			return false
		}
		if pairKey(node, slicer) != name {
			return false
		}
		value := pairValue(node)
		if value != nil && value.Kind == "object" {
			found = value
			return true
		}
		return false
	})
	return found
}

func collectObjectKeys(objectNode *jsparse.TSNode, slicer sourceSlicer) []string {
	if objectNode == nil {
		return nil
	}

	keys := make([]string, 0, len(objectNode.Children))
	for _, child := range objectNode.Children {
		key := ""
		switch child.Kind {
		case "pair":
			key = pairKey(child, slicer)
		case "method_definition":
			key = objectMemberKey(child, slicer)
		}
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func pairKey(node *jsparse.TSNode, slicer sourceSlicer) string {
	for _, child := range node.Children {
		if isTrivia(child) || child.Kind == ":" {
			continue
		}
		return trimQuotes(strings.TrimSpace(slicer.text(child)))
	}
	return ""
}

func pairValue(node *jsparse.TSNode) *jsparse.TSNode {
	seenColon := false
	for _, child := range node.Children {
		if child.Kind == ":" {
			seenColon = true
			continue
		}
		if !seenColon || isTrivia(child) {
			continue
		}
		return child
	}
	return nil
}

func objectMemberKey(node *jsparse.TSNode, slicer sourceSlicer) string {
	for _, child := range node.Children {
		if isTrivia(child) {
			continue
		}
		return trimQuotes(strings.TrimSpace(slicer.text(child)))
	}
	return ""
}

func firstDescendantOfKind(root *jsparse.TSNode, kind string) *jsparse.TSNode {
	var found *jsparse.TSNode
	walk(root, func(node *jsparse.TSNode) bool {
		if node.Kind == kind {
			found = node
			return true
		}
		return false
	})
	return found
}

func walk(node *jsparse.TSNode, visit func(node *jsparse.TSNode) bool) bool {
	if node == nil {
		return false
	}
	if visit(node) {
		return true
	}
	for _, child := range node.Children {
		if walk(child, visit) {
			return true
		}
	}
	return false
}

func isTrivia(node *jsparse.TSNode) bool {
	return node == nil || node.Kind == "," || node.Kind == "(" || node.Kind == ")" || node.Kind == "{" || node.Kind == "}" || node.Kind == "comment"
}

func parseObjectNode(node *jsparse.TSNode, slicer sourceSlicer, out any) error {
	raw := slicer.text(node)
	data := jsObjectToJSON(raw)
	if err := json.Unmarshal([]byte(data), out); err != nil {
		return errors.Wrapf(err, "decoding object %s", raw)
	}
	return nil
}

func trimQuotes(value string) string {
	return strings.Trim(value, `"'`+"`")
}

func jsObjectToJSON(js string) string {
	var sb strings.Builder
	i := 0
	n := len(js)

	for i < n {
		ch := js[i]

		switch {
		case ch == '/' && i+1 < n && js[i+1] == '/':
			for i < n && js[i] != '\n' {
				i++
			}

		case ch == '/' && i+1 < n && js[i+1] == '*':
			i += 2
			for i+1 < n && (js[i] != '*' || js[i+1] != '/') {
				i++
			}
			i += 2

		case ch == '\'' || ch == '"':
			i = writeJSONStringToken(&sb, js, i)

		case isIdentifierStart(ch):
			start := i
			for i < n && isIdentifierPart(js[i]) {
				i++
			}
			token := js[start:i]

			j := i
			for j < n && (js[j] == ' ' || js[j] == '\n' || js[j] == '\t' || js[j] == '\r') {
				j++
			}

			if j < n && js[j] == ':' {
				sb.WriteByte('"')
				sb.WriteString(token)
				sb.WriteByte('"')
			} else {
				sb.WriteString(token)
			}

		default:
			sb.WriteByte(ch)
			i++
		}
	}

	return trailingCommaPattern.ReplaceAllString(sb.String(), "$1")
}

func writeJSONStringToken(sb *strings.Builder, js string, start int) int {
	quote := js[start]
	n := len(js)

	sb.WriteByte('"')
	for i := start + 1; i < n; i++ {
		ch := js[i]

		if ch == '\\' {
			if i+1 >= n {
				sb.WriteByte('\\')
				return n
			}

			next := js[i+1]
			switch next {
			case '\'':
				if quote == '\'' {
					sb.WriteByte('\'')
				} else {
					sb.WriteByte('\\')
					sb.WriteByte(next)
				}
			case '"':
				sb.WriteByte('\\')
				sb.WriteByte('"')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(next)
			}
			i++
			continue
		}

		if ch == quote {
			sb.WriteByte('"')
			return i + 1
		}
		if ch == '"' {
			sb.WriteByte('\\')
		}
		sb.WriteByte(ch)
	}

	sb.WriteByte('"')
	return n
}

func isIdentifierStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}

func isIdentifierPart(ch byte) bool {
	return isIdentifierStart(ch) || (ch >= '0' && ch <= '9')
}

func DebugString(output *GeneratedPack) string {
	return fmt.Sprintf("%s:%d cards", output.PackID, len(output.Cards))
}
