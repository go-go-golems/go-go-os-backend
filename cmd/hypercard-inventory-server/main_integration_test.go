package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/pinoweb"
)

type integrationNoopEngine struct{}

func (integrationNoopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type integrationNoopSink struct{}

func (integrationNoopSink) PublishEvent(events.Event) error { return nil }

func newIntegrationServer(t *testing.T) *httptest.Server {
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

	webchatSrv, err := webchat.NewServer(context.Background(), parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
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
	appMux.Handle("/", webchatSrv.UIHandler())

	return httptest.NewServer(appMux)
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
