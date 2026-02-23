package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	plzconfirmbackend "github.com/go-go-golems/plz-confirm/pkg/backend"
	v1 "github.com/go-go-golems/plz-confirm/proto/generated/go/plz_confirm/v1"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/pinoweb"
)

type integrationNoopEngine struct{}

func (integrationNoopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type integrationStructuredEngine struct{}

func (integrationStructuredEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	meta := events.EventMetadata{ID: uuid.New()}
	events.PublishEventToContext(ctx, events.NewStartEvent(meta))

	part1 := "Inventory snapshot generated.\n<hypercard:widget:v1>\n```yaml\ntype: report\n"
	cumulative := part1
	events.PublishEventToContext(ctx, events.NewPartialCompletionEvent(meta, part1, cumulative))

	part2 := "" +
		"title: Integration Widget\n" +
		"artifact:\n" +
		"  id: integration-widget\n" +
		"  data:\n" +
		"    totalItems: 10\n" +
		"```\n" +
		"</hypercard:widget:v1>"
	cumulative += part2
	events.PublishEventToContext(ctx, events.NewPartialCompletionEvent(meta, part2, cumulative))
	events.PublishEventToContext(ctx, events.NewFinalEvent(meta, cumulative))

	out := t.Clone()
	if out == nil {
		out = &turns.Turn{}
	}
	turns.AppendBlock(out, turns.NewAssistantTextBlock(cumulative))
	return out, nil
}

type integrationNoopSink struct{}

func (integrationNoopSink) PublishEvent(events.Event) error { return nil }

func newIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newIntegrationServerWithRouterOptions(t)
}

func newIntegrationServerWithRouterOptions(t *testing.T, extraOptions ...webchat.RouterOption) *httptest.Server {
	t.Helper()

	parsed := values.New()
	staticFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>inventory</body></html>")},
	}
	runtimeComposer := infruntime.RuntimeComposerFunc(func(_ context.Context, req infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
		runtimeKey := strings.TrimSpace(req.RuntimeKey)
		if runtimeKey == "" {
			runtimeKey = "inventory"
		}
		return infruntime.RuntimeArtifacts{
			Engine:             integrationNoopEngine{},
			Sink:               integrationNoopSink{},
			RuntimeKey:         runtimeKey,
			RuntimeFingerprint: "fp-" + runtimeKey,
			SeedSystemPrompt:   "seed",
		}, nil
	})

	pinoweb.RegisterInventoryHypercardExtensions()
	options := []webchat.RouterOption{
		webchat.WithRuntimeComposer(runtimeComposer),
		webchat.WithEventSinkWrapper(pinoweb.NewInventoryEventSinkWrapper(context.Background())),
	}
	options = append(options, extraOptions...)

	webchatSrv, err := webchat.NewServer(
		context.Background(),
		parsed,
		staticFS,
		options...,
	)
	require.NoError(t, err)

	resolver := pinoweb.NewStrictRequestResolver("inventory")
	chatHandler := webhttp.NewChatHandler(webchatSrv.ChatService(), resolver)
	wsHandler := webhttp.NewWSHandler(
		webchatSrv.StreamHub(),
		resolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)
	timelineLogger := log.With().Str("component", "inventory-chat-test").Str("route", "/api/timeline").Logger()
	timelineHandler := webhttp.NewTimelineHandler(webchatSrv.TimelineService(), timelineLogger)

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	appMux.Handle("/api/", webchatSrv.APIHandler())
	plzconfirmbackend.NewServer().Mount(appMux, "/confirm")
	appMux.Handle("/", webchatSrv.UIHandler())

	return httptest.NewServer(appMux)
}

func TestWSHandler_EmitsHypercardLifecycleEvents(t *testing.T) {
	parsed := values.New()
	staticFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>inventory</body></html>")},
	}
	runtimeComposer := infruntime.RuntimeComposerFunc(func(_ context.Context, req infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
		runtimeKey := strings.TrimSpace(req.RuntimeKey)
		if runtimeKey == "" {
			runtimeKey = "inventory"
		}
		return infruntime.RuntimeArtifacts{
			Engine:             integrationStructuredEngine{},
			Sink:               nil,
			RuntimeKey:         runtimeKey,
			RuntimeFingerprint: "fp-" + runtimeKey,
			SeedSystemPrompt:   "seed",
		}, nil
	})

	pinoweb.RegisterInventoryHypercardExtensions()
	webchatSrv, err := webchat.NewServer(
		context.Background(),
		parsed,
		staticFS,
		webchat.WithRuntimeComposer(runtimeComposer),
		webchat.WithEventSinkWrapper(pinoweb.NewInventoryEventSinkWrapper(context.Background())),
	)
	require.NoError(t, err)

	resolver := pinoweb.NewStrictRequestResolver("inventory")
	chatHandler := webhttp.NewChatHandler(webchatSrv.ChatService(), resolver)
	wsHandler := webhttp.NewWSHandler(
		webchatSrv.StreamHub(),
		resolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)
	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	appMux.Handle("/api/", webchatSrv.APIHandler())
	appMux.Handle("/", webchatSrv.UIHandler())

	srv := httptest.NewServer(appMux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=conv-progress-1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))

	reqBody := []byte(`{"prompt":"run integration structured flow","conv_id":"conv-progress-1"}`)
	resp, err := http.Post(srv.URL+"/chat", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	seenWidgetStart := false
	seenWidgetReady := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
		_, frame, readErr := conn.ReadMessage()
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				continue
			}
			break
		}
		switch integrationSemEventType(frame) {
		case "hypercard.widget.start":
			seenWidgetStart = true
		case "hypercard.widget.v1":
			seenWidgetReady = true
		}
		if seenWidgetStart && seenWidgetReady {
			break
		}
	}

	require.True(t, seenWidgetStart, "expected hypercard.widget.start over websocket")
	require.True(t, seenWidgetReady, "expected hypercard.widget.v1 over websocket")
}

func TestChatHandler_StartedResponse(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	reqBody := []byte(`{"prompt":"hello from integration","conv_id":"conv-int-1"}`)
	resp, err := http.Post(srv.URL+"/chat", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "started", payload["status"])
	require.Equal(t, "conv-int-1", payload["conv_id"])
	require.NotEmpty(t, payload["session_id"])
}

func TestWSHandler_RequiresConvID(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWSHandler_HelloAndPong(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=conv-ws-1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("ping")))

	seenPong := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !seenPong {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
		_, frame, readErr := conn.ReadMessage()
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				continue
			}
			require.NoError(t, readErr)
		}
		if integrationSemEventType(frame) == "ws.pong" {
			seenPong = true
		}
	}
	require.True(t, seenPong, "expected ws.pong response to ping")
}

func TestTimelineEndpoint_ReturnsSnapshot(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/timeline?conv_id=conv-timeline-1")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	_, ok := payload["convId"]
	require.True(t, ok, "expected timeline snapshot with convId")
}

func TestConfirmRoutes_CoexistWithChatAndTimelineRoutes(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	chatResp, err := http.Post(srv.URL+"/chat", "application/json", bytes.NewReader([]byte(`{"prompt":"hello","conv_id":"conv-confirm-coexist"}`)))
	require.NoError(t, err)
	defer chatResp.Body.Close()
	require.Equal(t, http.StatusOK, chatResp.StatusCode)

	confirmPayload := &v1.UIRequest{
		Type:      v1.WidgetType_confirm,
		SessionId: "global",
		Input: &v1.UIRequest_ConfirmInput{
			ConfirmInput: &v1.ConfirmInput{
				Title: "Deploy now?",
			},
		},
	}
	confirmBody, err := protojson.Marshal(confirmPayload)
	require.NoError(t, err)
	confirmResp, err := http.Post(srv.URL+"/confirm/api/requests", "application/json", bytes.NewReader(confirmBody))
	require.NoError(t, err)
	defer confirmResp.Body.Close()
	require.Equal(t, http.StatusCreated, confirmResp.StatusCode)

	timelineResp, err := http.Get(srv.URL + "/api/timeline?conv_id=conv-confirm-coexist")
	require.NoError(t, err)
	defer timelineResp.Body.Close()
	require.Equal(t, http.StatusOK, timelineResp.StatusCode)
}

func TestConfirmWS_PrefixedEndpointStreamsPendingRequests(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	created := integrationCreateConfirmRequest(t, srv.URL)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/confirm/ws?sessionId=global"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	eventType, eventReq := integrationReadConfirmWSEvent(t, conn)
	require.Equal(t, "new_request", eventType)
	require.Equal(t, created.GetId(), eventReq.GetId())
	require.Equal(t, v1.RequestStatus_pending, eventReq.GetStatus())
}

func TestChatHandler_PersistsTurnSnapshotsWhenTurnStoreConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	turnsPath := filepath.Join(tmpDir, "turns.db")
	turnsDSN, err := chatstore.SQLiteTurnDSNForFile(turnsPath)
	require.NoError(t, err)

	turnStore, err := chatstore.NewSQLiteTurnStore(turnsDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = turnStore.Close() })

	srv := newIntegrationServerWithRouterOptions(t, webchat.WithTurnStore(turnStore))
	defer srv.Close()

	const convID = "conv-turn-persist-1"
	reqBody := []byte(`{"prompt":"persist turn snapshot test","conv_id":"` + convID + `"}`)
	resp, err := http.Post(srv.URL+"/chat", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool {
		snapshots, listErr := turnStore.List(context.Background(), chatstore.TurnQuery{
			ConvID: convID,
			Limit:  20,
		})
		if listErr != nil {
			return false
		}
		return len(snapshots) > 0
	}, 6*time.Second, 100*time.Millisecond, "expected persisted turn snapshots")

	snapshots, err := turnStore.List(context.Background(), chatstore.TurnQuery{
		ConvID: convID,
		Limit:  20,
	})
	require.NoError(t, err)
	require.NotEmpty(t, snapshots)
	require.FileExists(t, turnsPath)

	seenFinalPhase := false
	for _, snapshot := range snapshots {
		if snapshot.Phase == "final" {
			seenFinalPhase = true
		}
		require.NotEmpty(t, strings.TrimSpace(snapshot.Payload))
	}
	require.True(t, seenFinalPhase, "expected at least one final turn snapshot")

	_, err = os.Stat(turnsPath)
	require.NoError(t, err)
}

func integrationSemEventType(frame []byte) string {
	var env struct {
		Event struct {
			Type string `json:"type"`
		} `json:"event"`
	}
	if err := json.Unmarshal(frame, &env); err != nil {
		return ""
	}
	return env.Event.Type
}

func integrationCreateConfirmRequest(t *testing.T, baseURL string) *v1.UIRequest {
	t.Helper()

	payload := &v1.UIRequest{
		Type:      v1.WidgetType_confirm,
		SessionId: "global",
		Input: &v1.UIRequest_ConfirmInput{
			ConfirmInput: &v1.ConfirmInput{
				Title: "Approve release?",
			},
		},
	}
	body, err := protojson.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/confirm/api/requests", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	req := &v1.UIRequest{}
	require.NoError(t, protojson.Unmarshal(raw, req))
	return req
}

func integrationReadConfirmWSEvent(t *testing.T, conn *websocket.Conn) (string, *v1.UIRequest) {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, frame, err := conn.ReadMessage()
	require.NoError(t, err)

	var env struct {
		Type    string          `json:"type"`
		Request json.RawMessage `json:"request"`
	}
	require.NoError(t, json.Unmarshal(frame, &env))

	req := &v1.UIRequest{}
	require.NoError(t, protojson.Unmarshal(env.Request, req))
	return env.Type, req
}
