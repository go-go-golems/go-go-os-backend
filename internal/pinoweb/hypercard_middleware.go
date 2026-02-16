package pinoweb

import (
	"context"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/google/uuid"
)

type InventoryArtifactPolicyConfig struct {
	Instructions string
}

type InventoryArtifactGeneratorConfig struct {
	RequireWidget bool
	RequireCard   bool
}

func defaultArtifactPolicyInstructions() string {
	return strings.TrimSpace("" +
		"When generating structured inventory artifacts, emit tags exactly in this format:\n\n" +
		"<hypercard:widget:v1>\n" +
		"```yaml\n" +
		"type: report | item | table\n" +
		"title: string\n" +
		"artifact:\n" +
		"  id: string\n" +
		"  data: {}\n" +
		"actions: []\n" +
		"```\n" +
		"</hypercard:widget:v1>\n\n" +
		"<hypercard:cardproposal:v1>\n" +
		"```yaml\n" +
		"template: reportViewer | itemViewer\n" +
		"title: string\n" +
		"artifact:\n" +
		"  id: string\n" +
		"  data: {}\n" +
		"window:\n" +
		"  dedupe_key: optional-string\n" +
		"```\n" +
		"</hypercard:cardproposal:v1>\n\n" +
		"Rules:\n" +
		"- First output a short plain-language summary sentence before any structured tag.\n" +
		"- Do not invent fallback payloads if you cannot produce valid YAML.\n" +
		"- Always provide non-empty title fields.\n" +
		"- Use only YAML inside the tags.\n")
}

func NewInventoryArtifactPolicyMiddleware(cfg InventoryArtifactPolicyConfig) middleware.Middleware {
	instructions := strings.TrimSpace(cfg.Instructions)
	if instructions == "" {
		instructions = defaultArtifactPolicyInstructions()
	}
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
			if t == nil {
				t = &turns.Turn{}
			}
			upsertPolicySystemBlock(t, instructions)
			return next(ctx, t)
		}
	}
}

func NewInventoryArtifactGeneratorMiddleware(cfg InventoryArtifactGeneratorConfig) middleware.Middleware {
	requireWidget := cfg.RequireWidget
	requireCard := cfg.RequireCard
	if !requireWidget && !requireCard {
		requireWidget = true
		requireCard = true
	}

	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
			out, err := next(ctx, t)
			if err != nil {
				return out, err
			}
			if out == nil {
				return out, nil
			}

			assistantText := assistantTextFromTurn(out)
			md := newEventMetadataFromTurn(out)
			baseID := strings.TrimSpace(out.ID)
			if baseID == "" {
				baseID = md.ID.String()
			}

			if requireWidget && !strings.Contains(assistantText, "<hypercard:widget:v1>") {
				events.PublishEventToContext(ctx, &HypercardWidgetErrorEvent{
					EventImpl: events.EventImpl{
						Type_:     eventTypeHypercardWidgetError,
						Metadata_: md,
					},
					ItemID: baseID + ":widget",
					Error:  "missing structured widget block <hypercard:widget:v1>",
				})
			}

			if requireCard && !strings.Contains(assistantText, "<hypercard:cardproposal:v1>") {
				events.PublishEventToContext(ctx, &HypercardCardErrorEvent{
					EventImpl: events.EventImpl{
						Type_:     eventTypeHypercardCardError,
						Metadata_: md,
					},
					ItemID: baseID + ":card",
					Error:  "missing structured card proposal block <hypercard:cardproposal:v1>",
				})
			}

			return out, nil
		}
	}
}

func upsertPolicySystemBlock(t *turns.Turn, instructions string) {
	if t == nil {
		return
	}

	for i := range t.Blocks {
		b := &t.Blocks[i]
		if b.Kind != turns.BlockKindSystem {
			continue
		}
		mwName, ok, err := turns.KeyBlockMetaMiddleware.Get(b.Metadata)
		if err != nil || !ok || mwName != hypercardPolicyMiddlewareName {
			continue
		}
		if b.Payload == nil {
			b.Payload = map[string]any{}
		}
		b.Payload[turns.PayloadKeyText] = instructions
		return
	}

	newBlock := turns.NewSystemTextBlock(instructions)
	_ = turns.KeyBlockMetaMiddleware.Set(&newBlock.Metadata, hypercardPolicyMiddlewareName)

	insertAt := len(t.Blocks)
	for i, b := range t.Blocks {
		if b.Kind == turns.BlockKindSystem {
			insertAt = i + 1
			break
		}
	}
	t.Blocks = append(t.Blocks, turns.Block{})
	copy(t.Blocks[insertAt+1:], t.Blocks[insertAt:])
	t.Blocks[insertAt] = newBlock
}

func assistantTextFromTurn(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	for _, block := range t.Blocks {
		if block.Kind != turns.BlockKindLLMText {
			continue
		}
		text, _ := block.Payload[turns.PayloadKeyText].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	return b.String()
}

func newEventMetadataFromTurn(t *turns.Turn) events.EventMetadata {
	md := events.EventMetadata{
		ID: uuid.New(),
	}
	if t == nil {
		return md
	}
	md.TurnID = strings.TrimSpace(t.ID)
	if sid, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
		md.SessionID = sid
	}
	if iid, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		md.InferenceID = iid
	}
	return md
}
