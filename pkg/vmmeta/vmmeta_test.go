package vmmeta

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateExtractsCardMetadataAndDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cardsDir := filepath.Join(root, "cards")
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(cardsDir, 0o755); err != nil {
		t.Fatalf("mkdir cards: %v", err)
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	writeTestFile(t, filepath.Join(cardsDir, "sprint.vm.js"), "__card__({ id: 'kanbanSprintBoard', packId: 'kanban.v1', title: 'Sprint Board', icon: '🏁' });\n"+
		"__doc__({ name: 'kanbanSprintBoard', summary: 'Sprint demo board' });\n"+
		"doc`\n"+
		"---\n"+
		"symbol: kanbanSprintBoard\n"+
		"---\n"+
		"Sprint board prose.\n"+
		"`;\n"+
		"defineCard('kanbanSprintBoard', ({ widgets }) => ({\n"+
		"  render() {\n"+
		"    return widgets.kanban.board({});\n"+
		"  },\n"+
		"  handlers: {\n"+
		"    saveTask(context, args) {\n"+
		"      return [context, args];\n"+
		"    },\n"+
		"    moveTask(context, args) {\n"+
		"      return [context, args];\n"+
		"    }\n"+
		"  }\n"+
		"}), 'kanban.v1');\n")

	writeTestFile(t, filepath.Join(docsDir, "kanban-pack.docs.vm.js"), "__package__({ name: 'kanban.v1', title: 'Kanban Runtime Pack', version: '1' });\n"+
		"__doc__('widgets.kanban.board', { summary: 'Build a kanban board tree' });\n"+
		"doc`\n"+
		"---\n"+
		"package: kanban.v1\n"+
		"---\n"+
		"Pack prose.\n"+
		"`;\n")

	output, err := Generate(context.Background(), GenerateOptions{
		PackID:   "kanban.v1",
		CardsDir: cardsDir,
		DocsDir:  docsDir,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if output.PackID != "kanban.v1" {
		t.Fatalf("unexpected pack id: %s", output.PackID)
	}
	if len(output.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(output.Cards))
	}

	card := output.Cards[0]
	if card.ID != "kanbanSprintBoard" {
		t.Fatalf("unexpected card id: %s", card.ID)
	}
	if card.Title != "Sprint Board" {
		t.Fatalf("unexpected title: %s", card.Title)
	}
	if got := strings.Join(card.HandlerNames, ","); got != "moveTask,saveTask" {
		t.Fatalf("unexpected handlers: %s", got)
	}
	if !strings.Contains(card.Source, "__card__") {
		t.Fatalf("expected original source to be preserved")
	}

	if output.Docs == nil || output.Docs.ByPackage["kanban.v1"] == nil {
		t.Fatalf("expected package docs to be extracted")
	}
	if output.Docs.BySymbol["kanbanSprintBoard"] == nil {
		t.Fatalf("expected card docs to be extracted")
	}
}

func TestGenerateRejectsMalformedCardSentinel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cardsDir := filepath.Join(root, "cards")
	if err := os.MkdirAll(cardsDir, 0o755); err != nil {
		t.Fatalf("mkdir cards: %v", err)
	}

	writeTestFile(t, filepath.Join(cardsDir, "broken.vm.js"), `__card__({ id: 'brokenCard', title: 'Broken' });
defineCard('brokenCard', () => ({ render() { return null; } }), 'kanban.v1');
`)

	_, err := Generate(context.Background(), GenerateOptions{
		PackID:   "kanban.v1",
		CardsDir: cardsDir,
	})
	if err == nil {
		t.Fatal("expected Generate to fail")
	}
	if !strings.Contains(err.Error(), "missing packId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateAndWriteIsDeterministic(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cardsDir := filepath.Join(root, "cards")
	docsDir := filepath.Join(root, "docs")
	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(cardsDir, 0o755); err != nil {
		t.Fatalf("mkdir cards: %v", err)
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	writeTestFile(t, filepath.Join(cardsDir, "b.vm.js"), `__card__({ id: 'bBoard', packId: 'kanban.v1', title: 'B', icon: 'B' });
defineCard('bBoard', () => ({ render() { return null; }, handlers: { ping() {} } }), 'kanban.v1');
`)
	writeTestFile(t, filepath.Join(cardsDir, "a.vm.js"), `__card__({ id: 'aBoard', packId: 'kanban.v1', title: 'A', icon: 'A' });
defineCard('aBoard', () => ({ render() { return null; }, handlers: { pong() {} } }), 'kanban.v1');
`)
	writeTestFile(t, filepath.Join(docsDir, "pack.vm.js"), `__package__({ name: 'kanban.v1', title: 'Kanban Runtime Pack' });`)

	jsonPath := filepath.Join(outDir, "vmmeta.json")
	tsPath := filepath.Join(outDir, "vmmeta.generated.ts")

	opts := GenerateOptions{
		PackID:     "kanban.v1",
		CardsDir:   cardsDir,
		DocsDir:    docsDir,
		OutputJSON: jsonPath,
		OutputTS:   tsPath,
	}

	if err := GenerateAndWrite(context.Background(), opts); err != nil {
		t.Fatalf("GenerateAndWrite first run: %v", err)
	}
	firstJSON := readTestFile(t, jsonPath)
	firstTS := readTestFile(t, tsPath)

	if err := GenerateAndWrite(context.Background(), opts); err != nil {
		t.Fatalf("GenerateAndWrite second run: %v", err)
	}
	secondJSON := readTestFile(t, jsonPath)
	secondTS := readTestFile(t, tsPath)

	if firstJSON != secondJSON {
		t.Fatal("JSON output changed between identical runs")
	}
	if firstTS != secondTS {
		t.Fatal("TypeScript output changed between identical runs")
	}
	if !strings.Contains(firstTS, "export const VM_PACK_METADATA") {
		t.Fatal("expected TS wrapper export")
	}
	if strings.Index(firstJSON, `"id": "aBoard"`) > strings.Index(firstJSON, `"id": "bBoard"`) {
		t.Fatal("expected cards to be sorted deterministically by id")
	}
}

func TestJSObjectToJSONPreservesQuotedStrings(t *testing.T) {
	t.Parallel()

	input := `{title: "alpha: // keep this", subtitle: 'It\'s "fine"', note: "x:y"}`
	data := jsObjectToJSON(input)

	var got map[string]string
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", data, err)
	}

	if got["title"] != "alpha: // keep this" {
		t.Fatalf("unexpected title: %q", got["title"])
	}
	if got["subtitle"] != `It's "fine"` {
		t.Fatalf("unexpected subtitle: %q", got["subtitle"])
	}
	if got["note"] != "x:y" {
		t.Fatalf("unexpected note: %q", got["note"])
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
