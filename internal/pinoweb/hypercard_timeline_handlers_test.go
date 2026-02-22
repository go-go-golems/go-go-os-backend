package pinoweb

import (
	"context"
	"encoding/json"
	"testing"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/stretchr/testify/require"
)

func semFrameForTest(t *testing.T, eventType, id string, seq uint64, data map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"sem": true,
		"event": map[string]any{
			"type":      eventType,
			"id":        id,
			"seq":       seq,
			"stream_id": "stream-1",
			"data":      data,
		},
	})
	require.NoError(t, err)
	return raw
}

func TestHypercardTimelineHandlers_SuggestionsProjectToSingleAssistantEntity(t *testing.T) {
	webchat.ClearTimelineHandlers()
	t.Cleanup(webchat.ClearTimelineHandlers)
	registerHypercardTimelineHandlers()

	store := chatstore.NewInMemoryTimelineStore(100)
	projector := webchat.NewTimelineProjector("conv-suggestions", store, nil)

	require.NoError(t, projector.ApplySemFrame(context.Background(), semFrameForTest(
		t,
		"hypercard.suggestions.start",
		"suggestions-turn-1",
		10,
		map[string]any{"suggestions": []string{"Show current inventory status", "What items are low stock?"}},
	)))
	require.NoError(t, projector.ApplySemFrame(context.Background(), semFrameForTest(
		t,
		"hypercard.suggestions.update",
		"suggestions-turn-1",
		11,
		map[string]any{"suggestions": []string{"Summarize today sales"}},
	)))
	require.NoError(t, projector.ApplySemFrame(context.Background(), semFrameForTest(
		t,
		"hypercard.suggestions.v1",
		"suggestions-turn-1",
		12,
		map[string]any{"suggestions": []string{"Summarize today sales", "Show margin report"}},
	)))

	snap, err := store.GetSnapshot(context.Background(), "conv-suggestions", 0, 100)
	require.NoError(t, err)
	require.Equal(t, uint64(12), snap.Version)
	require.Len(t, snap.Entities, 1)

	entity := snap.Entities[0]
	require.Equal(t, "suggestions:assistant", entity.Id)
	require.Equal(t, "suggestions", entity.Kind)
	require.NotNil(t, entity.Props)

	props := entity.Props.AsMap()
	require.Equal(t, "assistant", props["source"])
	require.Equal(t, nil, props["consumedAt"])
	require.Equal(t, []any{"Summarize today sales", "Show margin report"}, props["items"])
}
