package pinoweb

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/events/structuredsink"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type collectorSink struct {
	events []events.Event
}

func (c *collectorSink) PublishEvent(ev events.Event) error {
	c.events = append(c.events, ev)
	return nil
}

func countEventsByType(list []events.Event, t events.EventType) int {
	count := 0
	for _, ev := range list {
		if ev == nil {
			continue
		}
		if ev.Type() == t {
			count++
		}
	}
	return count
}

func TestWidgetExtractor_TitleGatedStartThenReady(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventoryWidgetExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	chunk1 := "<hypercard:widget:v1>\n```yaml\ntype: report\n"
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, chunk1, chunk1)))
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardWidgetStart))

	full := chunk1 +
		"title: Stock Summary\n" +
		"artifact:\n" +
		"  id: inv-report\n" +
		"  data:\n" +
		"    totalItems: 10\n" +
		"```\n" +
		"</hypercard:widget:v1>"
	chunk2 := strings.TrimPrefix(full, chunk1)
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, chunk2, full)))
	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardWidgetStart))

	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))
	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardWidgetV1))
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardWidgetError))
}

func TestWidgetExtractor_MalformedBlockEmitsError(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventoryWidgetExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	full := "<hypercard:widget:v1>\n```yaml\ntype: report\ntitle: Broken\n"
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, full, full)))
	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))

	require.GreaterOrEqual(t, countEventsByType(col.events, eventTypeHypercardWidgetError), 1)
}

func TestRuntimeCardExtractor_ValidPayload(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventoryRuntimeCardExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	full := "<hypercard:card:v2>\n```yaml\n" +
		"name: Low Stock Items\n" +
		"title: Items Below Reorder Threshold\n" +
		"artifact:\n" +
		"  id: low-stock-drilldown\n" +
		"  data:\n" +
		"    threshold: 5\n" +
		"card:\n" +
		"  id: lowStockDrilldown\n" +
		"  code: |-\n" +
		"    ({ ui }) => ({\n" +
		"      render({ globalState }) {\n" +
		"        return ui.panel([ui.text(\"Low Stock\")]);\n" +
		"      }\n" +
		"    })\n" +
		"```\n" +
		"</hypercard:card:v2>"

	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, full, full)))
	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))

	require.GreaterOrEqual(t, countEventsByType(col.events, eventTypeHypercardCardStart), 1)
	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardCardV2))
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardCardError))
}

func TestRuntimeCardExtractor_MissingCardCode(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventoryRuntimeCardExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	full := "<hypercard:card:v2>\n```yaml\n" +
		"name: Missing Code\n" +
		"title: A Card Without Code\n" +
		"artifact:\n" +
		"  id: missing-code\n" +
		"  data: {}\n" +
		"card:\n" +
		"  id: missingCode\n" +
		"```\n" +
		"</hypercard:card:v2>"

	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, full, full)))
	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))

	require.GreaterOrEqual(t, countEventsByType(col.events, eventTypeHypercardCardError), 1)
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardCardV2))
}

func TestRuntimeCardExtractor_MissingCardId(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventoryRuntimeCardExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	full := "<hypercard:card:v2>\n```yaml\n" +
		"name: Missing ID\n" +
		"title: A Card Without ID\n" +
		"artifact:\n" +
		"  id: missing-id\n" +
		"  data: {}\n" +
		"card:\n" +
		"  code: |-\n" +
		"    ({ ui }) => ({ render() { return ui.text(\"hi\"); } })\n" +
		"```\n" +
		"</hypercard:card:v2>"

	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, full, full)))
	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))

	require.GreaterOrEqual(t, countEventsByType(col.events, eventTypeHypercardCardError), 1)
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardCardV2))
}

func TestRuntimeCardExtractor_StreamingName(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventoryRuntimeCardExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	// First chunk: just name and title, no code yet
	chunk1 := "<hypercard:card:v2>\n```yaml\nname: Low Stock Items\ntitle: Items Below Threshold\n"
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, chunk1, chunk1)))
	// Name should be available, so card.start should fire
	require.GreaterOrEqual(t, countEventsByType(col.events, eventTypeHypercardCardStart), 1)

	// Full payload
	full := chunk1 +
		"artifact:\n" +
		"  id: low-stock\n" +
		"  data: {}\n" +
		"card:\n" +
		"  id: lowStock\n" +
		"  code: |-\n" +
		"    ({ ui }) => ({ render() { return ui.text(\"hi\"); } })\n" +
		"```\n" +
		"</hypercard:card:v2>"
	chunk2 := strings.TrimPrefix(full, chunk1)
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, chunk2, full)))
	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))

	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardCardV2))
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardCardError))
}

func TestSuggestionsExtractor_ProgressiveStartUpdateReady(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	sink := structuredsink.NewFilteringSink(col, structuredsink.Options{
		Malformed: structuredsink.MalformedErrorEvents,
	}, &inventorySuggestionsExtractor{})

	meta := events.EventMetadata{ID: uuid.New()}
	chunk1 := "<hypercard:suggestions:v1>\n```yaml\nsuggestions:\n  - Show current inventory status\n"
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, chunk1, chunk1)))
	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardSuggestionsStart))

	full := chunk1 +
		"  - What items are low stock?\n" +
		"  - Summarize today sales\n" +
		"```\n" +
		"</hypercard:suggestions:v1>"
	chunk2 := strings.TrimPrefix(full, chunk1)
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, chunk2, full)))
	require.GreaterOrEqual(t, countEventsByType(col.events, eventTypeHypercardSuggestionsUpdate), 1)

	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, full)))
	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardSuggestionsV1))
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardSuggestionsError))
}

func TestArtifactGeneratorMiddleware_EmitsMissingErrorsNoFallback(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	ctx := events.WithEventSinks(context.Background(), col)
	mw := NewInventoryArtifactGeneratorMiddleware(InventoryArtifactGeneratorConfig{
		RequireWidget: true,
		RequireCard:   true,
	})

	turn := &turns.Turn{
		ID: "turn-1",
		Blocks: []turns.Block{
			turns.NewAssistantTextBlock("No structured output here."),
		},
	}
	_, err := mw(func(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
		return t, nil
	})(ctx, turn)
	require.NoError(t, err)

	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardWidgetError))
	require.Equal(t, 1, countEventsByType(col.events, eventTypeHypercardCardError))
}

func TestArtifactGeneratorMiddleware_NoMissingErrorsWhenTagsPresent(t *testing.T) {
	RegisterInventoryHypercardExtensions()

	col := &collectorSink{}
	ctx := events.WithEventSinks(context.Background(), col)
	mw := NewInventoryArtifactGeneratorMiddleware(InventoryArtifactGeneratorConfig{
		RequireWidget: true,
		RequireCard:   true,
	})

	turn := &turns.Turn{
		ID: "turn-2",
		Blocks: []turns.Block{
			turns.NewAssistantTextBlock("<hypercard:widget:v1></hypercard:widget:v1>\n<hypercard:card:v2></hypercard:card:v2>"),
		},
	}
	_, err := mw(func(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
		return t, nil
	})(ctx, turn)
	require.NoError(t, err)

	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardWidgetError))
	require.Equal(t, 0, countEventsByType(col.events, eventTypeHypercardCardError))
}
