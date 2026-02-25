package pinoweb

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/events/structuredsink"
	"github.com/go-go-golems/geppetto/pkg/events/structuredsink/parsehelpers"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/pkg/errors"
)

type inventoryWidgetExtractor struct{}

func (e *inventoryWidgetExtractor) TagPackage() string { return "hypercard" }
func (e *inventoryWidgetExtractor) TagType() string    { return "widget" }
func (e *inventoryWidgetExtractor) TagVersion() string { return "v1" }
func (e *inventoryWidgetExtractor) NewSession(ctx context.Context, _ events.EventMetadata, itemID string) structuredsink.ExtractorSession {
	return &inventoryWidgetSession{
		ctx:    ctx,
		itemID: itemID,
		ctrl: parsehelpers.NewDebouncedYAML[inventoryWidgetPayload](parsehelpers.DebounceConfig{
			SnapshotEveryBytes: 256,
			SnapshotOnNewline:  true,
			ParseTimeout:       25 * time.Millisecond,
			MaxBytes:           64 << 10,
		}),
	}
}

type inventoryRuntimeCardExtractor struct{}

func (e *inventoryRuntimeCardExtractor) TagPackage() string { return "hypercard" }
func (e *inventoryRuntimeCardExtractor) TagType() string    { return "card" }
func (e *inventoryRuntimeCardExtractor) TagVersion() string { return "v2" }
func (e *inventoryRuntimeCardExtractor) NewSession(ctx context.Context, _ events.EventMetadata, itemID string) structuredsink.ExtractorSession {
	return &inventoryRuntimeCardSession{
		ctx:    ctx,
		itemID: itemID,
		ctrl: parsehelpers.NewDebouncedYAML[inventoryRuntimeCardPayload](parsehelpers.DebounceConfig{
			SnapshotEveryBytes: 256,
			SnapshotOnNewline:  true,
			ParseTimeout:       25 * time.Millisecond,
			MaxBytes:           128 << 10, // 128KB — card.code can be larger
		}),
	}
}

type inventorySuggestionsExtractor struct{}

func (e *inventorySuggestionsExtractor) TagPackage() string { return "hypercard" }
func (e *inventorySuggestionsExtractor) TagType() string    { return "suggestions" }
func (e *inventorySuggestionsExtractor) TagVersion() string { return "v1" }
func (e *inventorySuggestionsExtractor) NewSession(ctx context.Context, _ events.EventMetadata, itemID string) structuredsink.ExtractorSession {
	return &inventorySuggestionsSession{
		ctx:    ctx,
		itemID: itemID,
		ctrl: parsehelpers.NewDebouncedYAML[inventorySuggestionsPayload](parsehelpers.DebounceConfig{
			SnapshotEveryBytes: 128,
			SnapshotOnNewline:  true,
			ParseTimeout:       25 * time.Millisecond,
			MaxBytes:           32 << 10,
		}),
	}
}

type inventoryArtifactPayload struct {
	ID   string         `yaml:"id" json:"id"`
	Data map[string]any `yaml:"data" json:"data"`
}

type inventoryWidgetPayload struct {
	Type     string                   `yaml:"type" json:"type"`
	Title    string                   `yaml:"title" json:"title"`
	Artifact inventoryArtifactPayload `yaml:"artifact" json:"artifact"`
	Actions  []map[string]any         `yaml:"actions" json:"actions"`
}

type inventoryRuntimeCardPayload struct {
	Name     string                   `yaml:"name" json:"name"`
	Title    string                   `yaml:"title" json:"title"`
	Artifact inventoryArtifactPayload `yaml:"artifact" json:"artifact"`
	Card     struct {
		ID   string `yaml:"id" json:"id"`
		Code string `yaml:"code" json:"code"`
	} `yaml:"card" json:"card"`
}

type inventorySuggestionsPayload struct {
	Suggestions []string `yaml:"suggestions" json:"suggestions"`
}

type inventoryWidgetSession struct {
	ctx       context.Context
	itemID    string
	ctrl      *parsehelpers.YAMLController[inventoryWidgetPayload]
	started   bool
	lastValid *inventoryWidgetPayload
}

func (s *inventoryWidgetSession) OnStart(context.Context) []events.Event {
	return nil
}

func (s *inventoryWidgetSession) OnRaw(ctx context.Context, chunk []byte) []events.Event {
	if s.ctrl == nil {
		return nil
	}
	snap, err := s.ctrl.FeedBytes(chunk)
	if err != nil || snap == nil {
		return nil
	}
	s.lastValid = snap
	title := strings.TrimSpace(snap.Title)
	if title == "" {
		return nil
	}

	evs := []events.Event{}
	if !s.started {
		s.started = true
		evs = append(evs, &HypercardWidgetStartEvent{
			EventImpl:  events.EventImpl{Type_: eventTypeHypercardWidgetStart},
			ItemID:     s.itemID,
			Title:      title,
			WidgetType: strings.TrimSpace(snap.Type),
		})
		return evs
	}

	evs = append(evs, &HypercardWidgetUpdateEvent{
		EventImpl:  events.EventImpl{Type_: eventTypeHypercardWidgetUpdate},
		ItemID:     s.itemID,
		Title:      title,
		WidgetType: strings.TrimSpace(snap.Type),
		Data:       payloadToMap(snap),
	})
	return evs
}

func (s *inventoryWidgetSession) OnCompleted(ctx context.Context, raw []byte, success bool, err error) []events.Event {
	evs := []events.Event{}
	if err != nil {
		return []events.Event{&HypercardWidgetErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError},
			ItemID:    s.itemID,
			Error:     err.Error(),
		}}
	}
	if s.ctrl == nil {
		return []events.Event{&HypercardWidgetErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError},
			ItemID:    s.itemID,
			Error:     "widget parser not initialized",
		}}
	}
	snap, parseErr := s.ctrl.FinalBytes(raw)
	if parseErr != nil {
		return []events.Event{&HypercardWidgetErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError},
			ItemID:    s.itemID,
			Error:     parseErr.Error(),
		}}
	}
	if snap == nil {
		return []events.Event{&HypercardWidgetErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError},
			ItemID:    s.itemID,
			Error:     "missing widget payload",
		}}
	}
	title := strings.TrimSpace(snap.Title)
	if title == "" {
		return []events.Event{&HypercardWidgetErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError},
			ItemID:    s.itemID,
			Error:     "widget title is required",
		}}
	}
	widgetType := strings.TrimSpace(snap.Type)
	if widgetType == "" {
		return []events.Event{&HypercardWidgetErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError},
			ItemID:    s.itemID,
			Error:     "widget type is required",
		}}
	}

	if !s.started {
		s.started = true
		evs = append(evs, &HypercardWidgetStartEvent{
			EventImpl:  events.EventImpl{Type_: eventTypeHypercardWidgetStart},
			ItemID:     s.itemID,
			Title:      title,
			WidgetType: widgetType,
		})
	}
	evs = append(evs, &HypercardWidgetReadyEvent{
		EventImpl:  events.EventImpl{Type_: eventTypeHypercardWidgetV1},
		ItemID:     s.itemID,
		Title:      title,
		WidgetType: widgetType,
		Data:       payloadToMap(snap),
	})
	return evs
}

type inventoryRuntimeCardSession struct {
	ctx       context.Context
	itemID    string
	ctrl      *parsehelpers.YAMLController[inventoryRuntimeCardPayload]
	started   bool
	lastValid *inventoryRuntimeCardPayload
}

func (s *inventoryRuntimeCardSession) OnStart(context.Context) []events.Event {
	return nil
}

func (s *inventoryRuntimeCardSession) OnRaw(ctx context.Context, chunk []byte) []events.Event {
	if s.ctrl == nil {
		return nil
	}
	snap, err := s.ctrl.FeedBytes(chunk)
	if err != nil || snap == nil {
		return nil
	}
	s.lastValid = snap
	name := strings.TrimSpace(snap.Name)
	title := strings.TrimSpace(snap.Title)
	displayName := name
	if displayName == "" {
		displayName = title
	}
	if displayName == "" {
		return nil
	}

	evs := []events.Event{}
	if !s.started {
		s.started = true
		evs = append(evs, &HypercardCardStartEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardStart},
			ItemID:    s.itemID,
			Title:     displayName,
			Name:      name,
		})
		return evs
	}

	evs = append(evs, &HypercardCardUpdateEvent{
		EventImpl: events.EventImpl{Type_: eventTypeHypercardCardUpdate},
		ItemID:    s.itemID,
		Title:     displayName,
		Name:      name,
		Data:      payloadToMap(snap),
	})
	return evs
}

func (s *inventoryRuntimeCardSession) OnCompleted(ctx context.Context, raw []byte, success bool, err error) []events.Event {
	evs := []events.Event{}
	if err != nil {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     err.Error(),
		}}
	}
	if s.ctrl == nil {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     "runtime card parser not initialized",
		}}
	}
	snap, parseErr := s.ctrl.FinalBytes(raw)
	if parseErr != nil {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     parseErr.Error(),
		}}
	}
	if snap == nil {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     "missing runtime card payload",
		}}
	}
	name := strings.TrimSpace(snap.Name)
	title := strings.TrimSpace(snap.Title)
	displayName := name
	if displayName == "" {
		displayName = title
	}
	if displayName == "" {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     "runtime card name is required",
		}}
	}
	cardID := strings.TrimSpace(snap.Card.ID)
	if cardID == "" {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     "runtime card.id is required",
		}}
	}
	cardCode := strings.TrimSpace(snap.Card.Code)
	if cardCode == "" {
		return []events.Event{&HypercardCardErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError},
			ItemID:    s.itemID,
			Error:     "runtime card.code is required",
		}}
	}

	if !s.started {
		s.started = true
		evs = append(evs, &HypercardCardStartEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardCardStart},
			ItemID:    s.itemID,
			Title:     displayName,
			Name:      name,
		})
	}
	evs = append(evs, &HypercardCardV2ReadyEvent{
		EventImpl: events.EventImpl{Type_: eventTypeHypercardCardV2},
		ItemID:    s.itemID,
		Title:     title,
		Name:      name,
		Data:      payloadToMap(snap),
	})
	return evs
}

type inventorySuggestionsSession struct {
	ctx     context.Context
	itemID  string
	ctrl    *parsehelpers.YAMLController[inventorySuggestionsPayload]
	started bool
}

func (s *inventorySuggestionsSession) OnStart(context.Context) []events.Event {
	return nil
}

func (s *inventorySuggestionsSession) OnRaw(ctx context.Context, chunk []byte) []events.Event {
	if s.ctrl == nil {
		return nil
	}
	snap, err := s.ctrl.FeedBytes(chunk)
	if err != nil || snap == nil {
		return nil
	}
	suggestions := normalizeSuggestions(snap.Suggestions)
	if len(suggestions) == 0 {
		return nil
	}
	if !s.started {
		s.started = true
		return []events.Event{&HypercardSuggestionsStartEvent{
			EventImpl:   events.EventImpl{Type_: eventTypeHypercardSuggestionsStart},
			ItemID:      s.itemID,
			Suggestions: suggestions,
		}}
	}
	return []events.Event{&HypercardSuggestionsUpdateEvent{
		EventImpl:   events.EventImpl{Type_: eventTypeHypercardSuggestionsUpdate},
		ItemID:      s.itemID,
		Suggestions: suggestions,
	}}
}

func (s *inventorySuggestionsSession) OnCompleted(ctx context.Context, raw []byte, success bool, err error) []events.Event {
	if err != nil {
		return []events.Event{&HypercardSuggestionsErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardSuggestionsError},
			ItemID:    s.itemID,
			Error:     err.Error(),
		}}
	}
	if s.ctrl == nil {
		return nil
	}
	snap, parseErr := s.ctrl.FinalBytes(raw)
	if parseErr != nil {
		return []events.Event{&HypercardSuggestionsErrorEvent{
			EventImpl: events.EventImpl{Type_: eventTypeHypercardSuggestionsError},
			ItemID:    s.itemID,
			Error:     parseErr.Error(),
		}}
	}
	if snap == nil {
		return nil
	}
	suggestions := normalizeSuggestions(snap.Suggestions)
	if len(suggestions) == 0 {
		return nil
	}
	evs := []events.Event{}
	if !s.started {
		s.started = true
		evs = append(evs, &HypercardSuggestionsStartEvent{
			EventImpl:   events.EventImpl{Type_: eventTypeHypercardSuggestionsStart},
			ItemID:      s.itemID,
			Suggestions: suggestions,
		})
	}
	evs = append(evs, &HypercardSuggestionsReadyEvent{
		EventImpl:   events.EventImpl{Type_: eventTypeHypercardSuggestionsV1},
		ItemID:      s.itemID,
		Suggestions: suggestions,
	})
	return evs
}

func normalizeSuggestions(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, v)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func payloadToMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}
	}
	return m
}

func NewInventoryEventSinkWrapper(baseCtx context.Context) webchat.EventSinkWrapper {
	ctx := baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	return func(convID string, req infruntime.ConversationRuntimeRequest, sink events.EventSink) (events.EventSink, error) {
		if sink == nil {
			return nil, errors.New("event sink is nil")
		}
		return structuredsink.NewFilteringSinkWithContext(
			ctx,
			sink,
			structuredsink.Options{
				Malformed: structuredsink.MalformedErrorEvents,
				Debug:     false,
			},
			&inventoryWidgetExtractor{},
			&inventoryRuntimeCardExtractor{},
			&inventorySuggestionsExtractor{},
		), nil
	}
}
