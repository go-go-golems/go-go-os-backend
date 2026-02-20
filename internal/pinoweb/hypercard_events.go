package pinoweb

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/events"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	semregistry "github.com/go-go-golems/pinocchio/pkg/sem/registry"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	eventTypeHypercardWidgetStart       events.EventType = "hypercard.widget.start"
	eventTypeHypercardWidgetUpdate      events.EventType = "hypercard.widget.update"
	eventTypeHypercardWidgetV1          events.EventType = "hypercard.widget.v1"
	eventTypeHypercardWidgetError       events.EventType = "hypercard.widget.error"
	eventTypeHypercardSuggestionsStart  events.EventType = "hypercard.suggestions.start"
	eventTypeHypercardSuggestionsUpdate events.EventType = "hypercard.suggestions.update"
	eventTypeHypercardSuggestionsV1     events.EventType = "hypercard.suggestions.v1"
	eventTypeHypercardSuggestionsError  events.EventType = "hypercard.suggestions.error"
	eventTypeHypercardCardStart         events.EventType = "hypercard.card.start"
	eventTypeHypercardCardUpdate        events.EventType = "hypercard.card.update"
	eventTypeHypercardCardV2            events.EventType = "hypercard.card.v2"
	eventTypeHypercardCardError         events.EventType = "hypercard.card.error"
	hypercardPolicyMiddlewareName                        = "inventory_artifact_policy"
	hypercardGeneratorMiddlewareName                     = "inventory_artifact_generator"
	hypercardSuggestionsMiddlewareName                   = "inventory_suggestions_policy"
)

type HypercardWidgetStartEvent struct {
	events.EventImpl
	ItemID     string `json:"item_id"`
	Title      string `json:"title,omitempty"`
	WidgetType string `json:"widget_type,omitempty"`
}

type HypercardWidgetUpdateEvent struct {
	events.EventImpl
	ItemID     string `json:"item_id"`
	Title      string `json:"title,omitempty"`
	WidgetType string `json:"widget_type,omitempty"`
	Data       any    `json:"data,omitempty"`
}

type HypercardWidgetReadyEvent struct {
	events.EventImpl
	ItemID     string `json:"item_id"`
	Title      string `json:"title,omitempty"`
	WidgetType string `json:"widget_type,omitempty"`
	Data       any    `json:"data,omitempty"`
}

type HypercardWidgetErrorEvent struct {
	events.EventImpl
	ItemID string `json:"item_id"`
	Error  string `json:"error"`
}

type HypercardSuggestionsStartEvent struct {
	events.EventImpl
	ItemID      string   `json:"item_id"`
	Suggestions []string `json:"suggestions"`
}

type HypercardSuggestionsUpdateEvent struct {
	events.EventImpl
	ItemID      string   `json:"item_id"`
	Suggestions []string `json:"suggestions"`
}

type HypercardSuggestionsReadyEvent struct {
	events.EventImpl
	ItemID      string   `json:"item_id"`
	Suggestions []string `json:"suggestions"`
}

type HypercardSuggestionsErrorEvent struct {
	events.EventImpl
	ItemID string `json:"item_id"`
	Error  string `json:"error"`
}

type HypercardCardStartEvent struct {
	events.EventImpl
	ItemID string `json:"item_id"`
	Title  string `json:"title,omitempty"`
	Name   string `json:"name,omitempty"`
}

type HypercardCardUpdateEvent struct {
	events.EventImpl
	ItemID string `json:"item_id"`
	Title  string `json:"title,omitempty"`
	Name   string `json:"name,omitempty"`
	Data   any    `json:"data,omitempty"`
}

type HypercardCardV2ReadyEvent struct {
	events.EventImpl
	ItemID string `json:"item_id"`
	Title  string `json:"title,omitempty"`
	Name   string `json:"name,omitempty"`
	Data   any    `json:"data,omitempty"`
}

type HypercardCardErrorEvent struct {
	events.EventImpl
	ItemID string `json:"item_id"`
	Error  string `json:"error"`
}

var registerHypercardExtensionsOnce sync.Once

func RegisterInventoryHypercardExtensions() {
	registerHypercardExtensionsOnce.Do(func() {
		registerHypercardEventFactories()
		registerHypercardSEMMappings()
		registerHypercardTimelineHandlers()
	})
}

func registerHypercardEventFactories() {
	_ = events.RegisterEventFactory(string(eventTypeHypercardWidgetStart), func() events.Event {
		return &HypercardWidgetStartEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetStart}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardWidgetUpdate), func() events.Event {
		return &HypercardWidgetUpdateEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetUpdate}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardWidgetV1), func() events.Event {
		return &HypercardWidgetReadyEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetV1}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardWidgetError), func() events.Event {
		return &HypercardWidgetErrorEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardWidgetError}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardSuggestionsStart), func() events.Event {
		return &HypercardSuggestionsStartEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardSuggestionsStart}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardSuggestionsUpdate), func() events.Event {
		return &HypercardSuggestionsUpdateEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardSuggestionsUpdate}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardSuggestionsV1), func() events.Event {
		return &HypercardSuggestionsReadyEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardSuggestionsV1}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardSuggestionsError), func() events.Event {
		return &HypercardSuggestionsErrorEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardSuggestionsError}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardCardStart), func() events.Event {
		return &HypercardCardStartEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardCardStart}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardCardUpdate), func() events.Event {
		return &HypercardCardUpdateEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardCardUpdate}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardCardV2), func() events.Event {
		return &HypercardCardV2ReadyEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardCardV2}}
	})
	_ = events.RegisterEventFactory(string(eventTypeHypercardCardError), func() events.Event {
		return &HypercardCardErrorEvent{EventImpl: events.EventImpl{Type_: eventTypeHypercardCardError}}
	})
}

func registerHypercardSEMMappings() {
	semregistry.RegisterByType[*HypercardWidgetStartEvent](func(ev *HypercardWidgetStartEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.widget.start", ev.ItemID, map[string]any{
			"itemId":     ev.ItemID,
			"title":      ev.Title,
			"widgetType": ev.WidgetType,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardWidgetUpdateEvent](func(ev *HypercardWidgetUpdateEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.widget.update", ev.ItemID, map[string]any{
			"itemId":     ev.ItemID,
			"title":      ev.Title,
			"widgetType": ev.WidgetType,
			"data":       ev.Data,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardWidgetReadyEvent](func(ev *HypercardWidgetReadyEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.widget.v1", ev.ItemID, map[string]any{
			"itemId":     ev.ItemID,
			"title":      ev.Title,
			"widgetType": ev.WidgetType,
			"data":       ev.Data,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardWidgetErrorEvent](func(ev *HypercardWidgetErrorEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.widget.error", ev.ItemID, map[string]any{
			"itemId": ev.ItemID,
			"error":  ev.Error,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardSuggestionsStartEvent](func(ev *HypercardSuggestionsStartEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.suggestions.start", ev.ItemID, map[string]any{
			"itemId":      ev.ItemID,
			"suggestions": ev.Suggestions,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardSuggestionsUpdateEvent](func(ev *HypercardSuggestionsUpdateEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.suggestions.update", ev.ItemID, map[string]any{
			"itemId":      ev.ItemID,
			"suggestions": ev.Suggestions,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardSuggestionsReadyEvent](func(ev *HypercardSuggestionsReadyEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.suggestions.v1", ev.ItemID, map[string]any{
			"itemId":      ev.ItemID,
			"suggestions": ev.Suggestions,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardSuggestionsErrorEvent](func(ev *HypercardSuggestionsErrorEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.suggestions.error", ev.ItemID, map[string]any{
			"itemId": ev.ItemID,
			"error":  ev.Error,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardCardStartEvent](func(ev *HypercardCardStartEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.card.start", ev.ItemID, map[string]any{
			"itemId": ev.ItemID,
			"title":  ev.Title,
			"name":   ev.Name,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardCardUpdateEvent](func(ev *HypercardCardUpdateEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.card.update", ev.ItemID, map[string]any{
			"itemId": ev.ItemID,
			"title":  ev.Title,
			"name":   ev.Name,
			"data":   ev.Data,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardCardV2ReadyEvent](func(ev *HypercardCardV2ReadyEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.card.v2", ev.ItemID, map[string]any{
			"itemId": ev.ItemID,
			"title":  ev.Title,
			"name":   ev.Name,
			"data":   ev.Data,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
	semregistry.RegisterByType[*HypercardCardErrorEvent](func(ev *HypercardCardErrorEvent) ([][]byte, error) {
		frame, err := semFrame("hypercard.card.error", ev.ItemID, map[string]any{
			"itemId": ev.ItemID,
			"error":  ev.Error,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{frame}, nil
	})
}

func registerHypercardTimelineHandlers() {
	registerStatus := func(eventType string, statusType string, textFn func(map[string]any) string) {
		webchat.RegisterTimelineHandler(eventType, func(ctx context.Context, p *webchat.TimelineProjector, ev webchat.TimelineSemEvent, _ int64) error {
			data := parseTimelineData(ev.Data)
			entityID := ev.ID + ":status"
			return p.Upsert(ctx, ev.Seq, timelineEntityFromProtoMessage(entityID, "status", &timelinepb.StatusSnapshotV1{
				SchemaVersion: 1,
				Type:          statusType,
				Text:          textFn(data),
			}))
		})
	}

	registerStatus("hypercard.widget.start", "info", func(data map[string]any) string {
		title := stringFromMap(data, "title")
		if title == "" {
			return "Building widget..."
		}
		return "Building widget: " + title
	})
	registerStatus("hypercard.widget.update", "info", func(data map[string]any) string {
		title := stringFromMap(data, "title")
		if title == "" {
			return "Updating widget..."
		}
		return "Updating widget: " + title
	})
	registerStatus("hypercard.widget.error", "error", func(data map[string]any) string {
		msg := stringFromMap(data, "error")
		if msg == "" {
			msg = "Widget generation failed"
		}
		return msg
	})
	registerStatus("hypercard.card.start", "info", func(data map[string]any) string {
		title := stringFromMap(data, "title")
		if title == "" {
			return "Building card proposal..."
		}
		return "Building card proposal: " + title
	})
	registerStatus("hypercard.card.update", "info", func(data map[string]any) string {
		title := stringFromMap(data, "title")
		if title == "" {
			return "Updating card proposal..."
		}
		return "Updating card proposal: " + title
	})
	registerStatus("hypercard.card.error", "error", func(data map[string]any) string {
		msg := stringFromMap(data, "error")
		if msg == "" {
			msg = "Card proposal generation failed"
		}
		return msg
	})

	registerResult := func(eventType string, customKind string) {
		webchat.RegisterTimelineHandler(eventType, func(ctx context.Context, p *webchat.TimelineProjector, ev webchat.TimelineSemEvent, _ int64) error {
			data := parseTimelineData(ev.Data)
			resultStruct, err := structpb.NewStruct(data)
			if err != nil {
				resultStruct, _ = structpb.NewStruct(map[string]any{"raw": string(ev.Data)})
			}
			return p.Upsert(ctx, ev.Seq, timelineEntityFromProtoMessage(ev.ID+":result", "tool_result", &timelinepb.ToolResultSnapshotV1{
				SchemaVersion: 1,
				ToolCallId:    ev.ID,
				Result:        resultStruct,
				ResultRaw:     string(ev.Data),
				CustomKind:    customKind,
			}))
		})
	}

	registerResult("hypercard.widget.v1", "hypercard.widget.v1")
	registerResult("hypercard.card.v2", "hypercard.card.v2")
}

func parseTimelineData(raw json.RawMessage) map[string]any {
	var data map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	if data == nil {
		data = map[string]any{}
	}
	return data
}

func semFrame(eventType, id string, data map[string]any) ([]byte, error) {
	if strings.TrimSpace(id) == "" {
		id = "hypercard"
	}
	return json.Marshal(map[string]any{
		"sem": true,
		"event": map[string]any{
			"type": eventType,
			"id":   id,
			"data": data,
		},
	})
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func timelineEntityFromProtoMessage(id, kind string, msg proto.Message) *timelinepb.TimelineEntityV2 {
	return &timelinepb.TimelineEntityV2{
		Id:    strings.TrimSpace(id),
		Kind:  strings.TrimSpace(kind),
		Props: timelineStructFromProtoMessage(msg),
	}
}

func timelineStructFromProtoMessage(msg proto.Message) *structpb.Struct {
	if msg == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	raw, err := protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   false,
	}.Marshal(msg)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	st, err := structpb.NewStruct(m)
	if err != nil || st == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return st
}
