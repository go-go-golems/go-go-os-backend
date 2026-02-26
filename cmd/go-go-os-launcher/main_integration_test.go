package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	v1 "github.com/go-go-golems/plz-confirm/proto/generated/go/plz_confirm/v1"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/backendhost"
	"github.com/go-go-golems/hypercard-inventory-chat/internal/launcherui"
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

const integrationAppBasePath = "/api/apps/inventory"

func integrationChatPath() string           { return integrationAppBasePath + "/chat" }
func integrationWSPath() string             { return integrationAppBasePath + "/ws" }
func integrationTimelinePath() string       { return integrationAppBasePath + "/api/timeline" }
func integrationProfilesPath() string       { return integrationAppBasePath + "/api/chat/profiles" }
func integrationCurrentProfilePath() string { return integrationAppBasePath + "/api/chat/profile" }
func integrationConfirmPath() string        { return integrationAppBasePath + "/confirm" }
func integrationDebugConversationsPath() string {
	return integrationAppBasePath + "/api/debug/conversations"
}

func integrationProfilePath(slug string) string {
	return integrationProfilesPath() + "/" + strings.TrimSpace(slug)
}

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
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		runtimeKey := strings.TrimSpace(req.ProfileKey)
		if runtimeKey == "" {
			runtimeKey = "inventory"
		}
		systemPrompt := "seed"
		if req.ResolvedProfileRuntime != nil && strings.TrimSpace(req.ResolvedProfileRuntime.SystemPrompt) != "" {
			systemPrompt = strings.TrimSpace(req.ResolvedProfileRuntime.SystemPrompt)
		}
		allowedTools := append([]string(nil), inventoryToolNames...)
		if req.ResolvedProfileRuntime != nil && len(req.ResolvedProfileRuntime.Tools) > 0 {
			allowedTools = append([]string(nil), req.ResolvedProfileRuntime.Tools...)
		}
		versionedRuntimeKey := fmt.Sprintf("%s@v%d", runtimeKey, req.ProfileVersion)
		return infruntime.ComposedRuntime{
			Engine:             integrationNoopEngine{},
			Sink:               integrationNoopSink{},
			RuntimeKey:         versionedRuntimeKey,
			RuntimeFingerprint: "fp-" + versionedRuntimeKey + "-" + systemPrompt,
			SeedSystemPrompt:   systemPrompt,
			AllowedTools:       allowedTools,
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

	profileRegistry, err := newInMemoryProfileService("inventory", &gepprofiles.Profile{
		Slug:        gepprofiles.MustProfileSlug("inventory"),
		DisplayName: "Inventory",
		Description: "Tool-first inventory assistant profile.",
		Runtime: gepprofiles.RuntimeSpec{
			SystemPrompt: "You are an inventory assistant. Be concise, accurate, and tool-first.",
			Middlewares:  inventoryRuntimeMiddlewares(),
			Tools:        append([]string(nil), inventoryToolNames...),
		},
	}, &gepprofiles.Profile{
		Slug:        gepprofiles.MustProfileSlug("analyst"),
		DisplayName: "Analyst",
		Description: "Analysis-focused profile for inventory reporting.",
		Runtime: gepprofiles.RuntimeSpec{
			SystemPrompt: "You are an inventory analyst. Explain results with concise evidence.",
			Middlewares:  inventoryRuntimeMiddlewares(),
			Tools:        append([]string(nil), inventoryToolNames...),
		},
	})
	require.NoError(t, err)
	resolver := pinoweb.NewStrictRequestResolver("inventory").WithProfileRegistry(
		profileRegistry,
		gepprofiles.MustRegistrySlug(profileRegistrySlug),
	)

	moduleRegistry, err := backendhost.NewModuleRegistry(
		newInventoryBackendModule(
			webchatSrv,
			resolver,
			profileRegistry,
			nil,
			inventoryExtensionSchemas(),
		),
	)
	require.NoError(t, err)

	lifecycle := backendhost.NewLifecycleManager(moduleRegistry)
	require.NoError(t, lifecycle.Startup(context.Background(), backendhost.StartupOptions{
		RequiredAppIDs: []string{inventoryBackendAppID},
	}))
	t.Cleanup(func() { _ = lifecycle.Stop(context.Background()) })

	appMux := http.NewServeMux()
	backendhost.RegisterAppsManifestEndpoint(appMux, moduleRegistry)
	for _, module := range moduleRegistry.Modules() {
		manifest := module.Manifest()
		require.NoError(t, backendhost.MountNamespacedRoutes(appMux, manifest.AppID, module.MountRoutes))
	}
	registerLegacyAliasNotFoundHandlers(appMux)
	appMux.Handle("/", launcherui.Handler())

	return httptest.NewServer(appMux)
}

func TestWSHandler_EmitsHypercardLifecycleEvents(t *testing.T) {
	parsed := values.New()
	staticFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>inventory</body></html>")},
	}
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		runtimeKey := strings.TrimSpace(req.ProfileKey)
		if runtimeKey == "" {
			runtimeKey = "inventory"
		}
		return infruntime.ComposedRuntime{
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

	profileRegistry, err := newInMemoryProfileService("inventory", &gepprofiles.Profile{
		Slug:        gepprofiles.MustProfileSlug("inventory"),
		DisplayName: "Inventory",
		Runtime: gepprofiles.RuntimeSpec{
			SystemPrompt: "You are an inventory assistant.",
			Tools:        append([]string(nil), inventoryToolNames...),
		},
	})
	require.NoError(t, err)
	resolver := pinoweb.NewStrictRequestResolver("inventory").WithProfileRegistry(
		profileRegistry,
		gepprofiles.MustRegistrySlug(profileRegistrySlug),
	)

	moduleRegistry, err := backendhost.NewModuleRegistry(
		newInventoryBackendModule(
			webchatSrv,
			resolver,
			profileRegistry,
			nil,
			inventoryExtensionSchemas(),
		),
	)
	require.NoError(t, err)

	lifecycle := backendhost.NewLifecycleManager(moduleRegistry)
	require.NoError(t, lifecycle.Startup(context.Background(), backendhost.StartupOptions{
		RequiredAppIDs: []string{inventoryBackendAppID},
	}))
	defer func() { _ = lifecycle.Stop(context.Background()) }()

	appMux := http.NewServeMux()
	backendhost.RegisterAppsManifestEndpoint(appMux, moduleRegistry)
	for _, module := range moduleRegistry.Modules() {
		manifest := module.Manifest()
		require.NoError(t, backendhost.MountNamespacedRoutes(appMux, manifest.AppID, module.MountRoutes))
	}

	srv := httptest.NewServer(appMux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + integrationWSPath() + "?conv_id=conv-progress-1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))

	reqBody := []byte(`{"prompt":"run integration structured flow","conv_id":"conv-progress-1"}`)
	resp, err := http.Post(srv.URL+integrationChatPath(), "application/json", bytes.NewReader(reqBody))
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
	resp, err := http.Post(srv.URL+integrationChatPath(), "application/json", bytes.NewReader(reqBody))
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

	resp, err := http.Get(srv.URL + integrationWSPath())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWSHandler_HelloAndPong(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + integrationWSPath() + "?conv_id=conv-ws-1"
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

	resp, err := http.Get(srv.URL + integrationTimelinePath() + "?conv_id=conv-timeline-1")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	_, ok := payload["convId"]
	require.True(t, ok, "expected timeline snapshot with convId")
}

func TestOSAppsEndpoint_ListsInventoryModuleCapabilities(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/os/apps")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Apps []map[string]any `json:"apps"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(t, payload.Apps)

	inventoryFound := false
	for _, app := range payload.Apps {
		if appID, _ := app["app_id"].(string); appID == inventoryBackendAppID {
			inventoryFound = true
			require.Equal(t, "Inventory", app["name"])
			require.Equal(t, true, app["required"])
			require.Equal(t, true, app["healthy"])
			caps, ok := app["capabilities"].([]any)
			require.True(t, ok)
			require.Contains(t, caps, any("chat"))
			require.Contains(t, caps, any("profiles"))
		}
	}
	require.True(t, inventoryFound, "expected inventory backend module in /api/os/apps payload")
}

func TestLegacyAliasRoutes_AreNotMounted(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	legacyRoutes := []string{
		"/chat",
		"/ws?conv_id=legacy-1",
		"/api/timeline?conv_id=legacy-1",
	}
	for _, route := range legacyRoutes {
		resp, err := http.Get(srv.URL + route)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equalf(t, http.StatusNotFound, resp.StatusCode, "legacy route should not be mounted: %s", route)
	}
}

func TestChatHandler_PassesProfileDefaultMiddlewaresToRuntimeComposer(t *testing.T) {
	captured := make(chan infruntime.ConversationRuntimeRequest, 1)
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		captured <- req
		return infruntime.ComposedRuntime{
			Engine:             integrationNoopEngine{},
			Sink:               integrationNoopSink{},
			RuntimeKey:         "inventory",
			RuntimeFingerprint: "fp-inventory",
			SeedSystemPrompt:   "seed",
		}, nil
	})

	srv := newIntegrationServerWithRouterOptions(t, webchat.WithRuntimeComposer(runtimeComposer))
	defer srv.Close()

	reqBody := []byte(`{"prompt":"hello from integration","conv_id":"conv-int-profile-1","runtime_key":"inventory"}`)
	resp, err := http.Post(srv.URL+integrationChatPath(), "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case req := <-captured:
		require.NotNil(t, req.ResolvedProfileRuntime)
		require.GreaterOrEqual(t, len(req.ResolvedProfileRuntime.Middlewares), 2)
		require.Equal(t, "inventory_artifact_policy", req.ResolvedProfileRuntime.Middlewares[0].Name)
		require.Equal(t, "inventory_suggestions_policy", req.ResolvedProfileRuntime.Middlewares[1].Name)
	case <-time.After(2 * time.Second):
		t.Fatalf("did not capture runtime composer request")
	}
}

func TestProfileAPI_CRUDRoutesAreMounted(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	listResp, err := http.Get(srv.URL + integrationProfilesPath())
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	require.GreaterOrEqual(t, len(listed), 2)
	assertProfileListItemContract(t, listed[0])
	assertProfileListItemContract(t, listed[1])
	require.Equal(t, "analyst", listed[0]["slug"])
	require.Equal(t, "inventory", listed[1]["slug"])

	createResp, err := http.Post(srv.URL+integrationProfilesPath(), "application/json", strings.NewReader(`{
		"slug":"operator",
		"display_name":"Operator",
		"description":"Reads inventory data",
		"runtime":{"system_prompt":"You are an operator."},
		"extensions":{"Inventory.Starter_Suggestions@V1":{"items":["show low stock"]}},
		"set_default":true
	}`))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
	assertProfileDocumentContract(t, created)
	require.Equal(t, "operator", created["slug"])
	require.Equal(t, true, created["is_default"])

	getResp, err := http.Get(srv.URL + integrationProfilePath("operator"))
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)
	var got map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&got))
	assertProfileDocumentContract(t, got)
	require.Equal(t, "operator", got["slug"])
	extensions, ok := got["extensions"].(map[string]any)
	require.True(t, ok)
	_, ok = extensions["inventory.starter_suggestions@v1"]
	require.True(t, ok)

	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL+integrationProfilePath("operator"), strings.NewReader(`{
		"display_name":"Operator V2",
		"extensions":{"inventory.starter_suggestions@v1":{"items":["show aging inventory"]}},
		"expected_version":1
	}`))
	require.NoError(t, err)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	require.NoError(t, err)
	defer patchResp.Body.Close()
	require.Equal(t, http.StatusOK, patchResp.StatusCode)
	var patched map[string]any
	require.NoError(t, json.NewDecoder(patchResp.Body).Decode(&patched))
	assertProfileDocumentContract(t, patched)
	require.Equal(t, uint64(2), extractProfileVersion(patched))

	setDefaultResp, err := http.Post(srv.URL+integrationProfilePath("inventory")+"/default", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer setDefaultResp.Body.Close()
	require.Equal(t, http.StatusOK, setDefaultResp.StatusCode)
	var defaultDoc map[string]any
	require.NoError(t, json.NewDecoder(setDefaultResp.Body).Decode(&defaultDoc))
	assertProfileDocumentContract(t, defaultDoc)
	require.Equal(t, "inventory", defaultDoc["slug"])
	require.Equal(t, true, defaultDoc["is_default"])

	deleteReq, err := http.NewRequest(http.MethodDelete, srv.URL+integrationProfilePath("operator")+"?expected_version=2", nil)
	require.NoError(t, err)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	require.NoError(t, err)
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	getDeletedResp, err := http.Get(srv.URL + integrationProfilePath("operator"))
	require.NoError(t, err)
	defer getDeletedResp.Body.Close()
	require.Equal(t, http.StatusNotFound, getDeletedResp.StatusCode)
}

func TestProfileAPI_InvalidSlugAndRegistry_ReturnBadRequest(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	invalidRegistryResp, err := http.Get(srv.URL + integrationProfilesPath() + "?registry=invalid registry!")
	require.NoError(t, err)
	defer invalidRegistryResp.Body.Close()
	require.Equal(t, http.StatusBadRequest, invalidRegistryResp.StatusCode)

	invalidSlugResp, err := http.Post(
		srv.URL+integrationCurrentProfilePath(),
		"application/json",
		strings.NewReader(`{"slug":"not a valid slug!"}`),
	)
	require.NoError(t, err)
	defer invalidSlugResp.Body.Close()
	require.Equal(t, http.StatusBadRequest, invalidSlugResp.StatusCode)
}

func TestChatAPI_UnknownRegistrySelector_IsIgnored(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	reqBody := strings.NewReader(`{"prompt":"hello from unknown registry","conv_id":"conv-unknown-registry-1"}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+integrationChatPath()+"?registry_slug=missing", reqBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestConfirmRoutes_CoexistWithChatAndTimelineRoutes(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	chatResp, err := http.Post(srv.URL+integrationChatPath(), "application/json", bytes.NewReader([]byte(`{"prompt":"hello","conv_id":"conv-confirm-coexist"}`)))
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
	confirmResp, err := http.Post(srv.URL+integrationConfirmPath()+"/api/requests", "application/json", bytes.NewReader(confirmBody))
	require.NoError(t, err)
	defer confirmResp.Body.Close()
	require.Equal(t, http.StatusCreated, confirmResp.StatusCode)

	timelineResp, err := http.Get(srv.URL + integrationTimelinePath() + "?conv_id=conv-confirm-coexist")
	require.NoError(t, err)
	defer timelineResp.Body.Close()
	require.Equal(t, http.StatusOK, timelineResp.StatusCode)
}

func TestConfirmWS_PrefixedEndpointStreamsPendingRequests(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	created := integrationCreateConfirmRequest(t, srv.URL)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + integrationConfirmPath() + "/ws?sessionId=global"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	eventType, eventReq := integrationReadConfirmWSEvent(t, conn)
	require.Equal(t, "new_request", eventType)
	require.Equal(t, created.GetId(), eventReq.GetId())
	require.Equal(t, v1.RequestStatus_pending, eventReq.GetStatus())
}

func TestProfileE2E_ListSelectChat_RuntimeKeyReflectsSelection(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	listResp, err := http.Get(srv.URL + integrationProfilesPath())
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	require.True(t, hasProfileSlug(listed, "analyst"), "expected analyst profile in list")

	selectResp, err := http.Post(srv.URL+integrationCurrentProfilePath(), "application/json", strings.NewReader(`{"slug":"analyst"}`))
	require.NoError(t, err)
	defer selectResp.Body.Close()
	require.Equal(t, http.StatusOK, selectResp.StatusCode)
	cookie := mustProfileCookie(t, selectResp)

	const convID = "conv-profile-select-1"
	reqBody := strings.NewReader(`{"prompt":"hello analyst","conv_id":"` + convID + `"}`)
	chatReq, err := http.NewRequest(http.MethodPost, srv.URL+integrationChatPath(), reqBody)
	require.NoError(t, err)
	chatReq.Header.Set("Content-Type", "application/json")
	chatReq.AddCookie(cookie)
	chatResp, err := http.DefaultClient.Do(chatReq)
	require.NoError(t, err)
	defer chatResp.Body.Close()
	require.Equal(t, http.StatusOK, chatResp.StatusCode)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + integrationWSPath() + "?conv_id=" + convID
	wsHeaders := http.Header{}
	wsHeaders.Add("Cookie", cookie.String())
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeaders)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))
	require.Equal(t, "analyst@v0", integrationSemRuntimeKey(helloFrame))
}

func TestProfileE2E_SelectedProfileChange_RebuildsInFlightConversationRuntime(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	selectInventoryResp, err := http.Post(srv.URL+integrationCurrentProfilePath(), "application/json", strings.NewReader(`{"slug":"inventory"}`))
	require.NoError(t, err)
	defer selectInventoryResp.Body.Close()
	require.Equal(t, http.StatusOK, selectInventoryResp.StatusCode)
	inventoryCookie := mustProfileCookie(t, selectInventoryResp)

	const convID = "conv-profile-inflight-switch-1"
	chatReqInventory, err := http.NewRequest(
		http.MethodPost,
		srv.URL+integrationChatPath(),
		strings.NewReader(`{"prompt":"inventory baseline","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	chatReqInventory.Header.Set("Content-Type", "application/json")
	chatReqInventory.AddCookie(inventoryCookie)
	chatRespInventory, err := http.DefaultClient.Do(chatReqInventory)
	require.NoError(t, err)
	defer chatRespInventory.Body.Close()
	require.Equal(t, http.StatusOK, chatRespInventory.StatusCode)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + integrationWSPath() + "?conv_id=" + convID
	inventoryHeaders := http.Header{}
	inventoryHeaders.Add("Cookie", inventoryCookie.String())
	inventoryConn, _, err := websocket.DefaultDialer.Dial(wsURL, inventoryHeaders)
	require.NoError(t, err)
	require.NoError(t, inventoryConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, inventoryHelloFrame, err := inventoryConn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(inventoryHelloFrame))
	require.Equal(t, "inventory@v0", integrationSemRuntimeKey(inventoryHelloFrame))
	_ = inventoryConn.Close()

	selectAnalystResp, err := http.Post(srv.URL+integrationCurrentProfilePath(), "application/json", strings.NewReader(`{"slug":"analyst"}`))
	require.NoError(t, err)
	defer selectAnalystResp.Body.Close()
	require.Equal(t, http.StatusOK, selectAnalystResp.StatusCode)
	analystCookie := mustProfileCookie(t, selectAnalystResp)

	chatReqAnalyst, err := http.NewRequest(
		http.MethodPost,
		srv.URL+integrationChatPath(),
		strings.NewReader(`{"prompt":"switch to analyst","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	chatReqAnalyst.Header.Set("Content-Type", "application/json")
	chatReqAnalyst.AddCookie(analystCookie)
	chatRespAnalyst, err := http.DefaultClient.Do(chatReqAnalyst)
	require.NoError(t, err)
	defer chatRespAnalyst.Body.Close()
	require.Equal(t, http.StatusOK, chatRespAnalyst.StatusCode)

	analystHeaders := http.Header{}
	analystHeaders.Add("Cookie", analystCookie.String())
	analystConn, _, err := websocket.DefaultDialer.Dial(wsURL, analystHeaders)
	require.NoError(t, err)
	require.NoError(t, analystConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, analystHelloFrame, err := analystConn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(analystHelloFrame))
	require.Equal(t, "analyst@v0", integrationSemRuntimeKey(analystHelloFrame))
	_ = analystConn.Close()
}

func TestProfileE2E_RuntimeSwitchKeepsPerTurnRuntimeTruth(t *testing.T) {
	tmpDir := t.TempDir()
	turnsPath := filepath.Join(tmpDir, "turns-runtime-switch.db")
	turnsDSN, err := chatstore.SQLiteTurnDSNForFile(turnsPath)
	require.NoError(t, err)
	turnStore, err := chatstore.NewSQLiteTurnStore(turnsDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = turnStore.Close() })

	srv := newIntegrationServerWithRouterOptions(
		t,
		webchat.WithTurnStore(turnStore),
		webchat.WithDebugRoutesEnabled(true),
	)
	defer srv.Close()

	createPlannerResp, err := http.Post(srv.URL+integrationProfilesPath(), "application/json", strings.NewReader(`{
		"slug":"planner",
		"display_name":"Planner",
		"description":"Planning profile for runtime switch persistence test",
		"runtime":{"system_prompt":"You are a planner.","tools":["inventory.list","inventory.search"]}
	}`))
	require.NoError(t, err)
	defer createPlannerResp.Body.Close()
	require.Equal(t, http.StatusCreated, createPlannerResp.StatusCode)

	selectInventoryResp, err := http.Post(srv.URL+integrationCurrentProfilePath(), "application/json", strings.NewReader(`{"slug":"inventory"}`))
	require.NoError(t, err)
	defer selectInventoryResp.Body.Close()
	require.Equal(t, http.StatusOK, selectInventoryResp.StatusCode)
	inventoryCookie := mustProfileCookie(t, selectInventoryResp)

	const convID = "conv-runtime-truth-switch-1"
	reqInventory, err := http.NewRequest(
		http.MethodPost,
		srv.URL+integrationChatPath(),
		strings.NewReader(`{"prompt":"inventory baseline","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	reqInventory.Header.Set("Content-Type", "application/json")
	reqInventory.AddCookie(inventoryCookie)
	respInventory, err := http.DefaultClient.Do(reqInventory)
	require.NoError(t, err)
	defer respInventory.Body.Close()
	require.Contains(t, []int{http.StatusOK, http.StatusAccepted}, respInventory.StatusCode)

	selectPlannerResp, err := http.Post(srv.URL+integrationCurrentProfilePath(), "application/json", strings.NewReader(`{"slug":"planner"}`))
	require.NoError(t, err)
	defer selectPlannerResp.Body.Close()
	require.Equal(t, http.StatusOK, selectPlannerResp.StatusCode)
	plannerCookie := mustProfileCookie(t, selectPlannerResp)

	reqPlanner, err := http.NewRequest(
		http.MethodPost,
		srv.URL+integrationChatPath(),
		strings.NewReader(`{"prompt":"switch to planner","conv_id":"`+convID+`"}`),
	)
	require.NoError(t, err)
	reqPlanner.Header.Set("Content-Type", "application/json")
	reqPlanner.AddCookie(plannerCookie)
	respPlanner, err := http.DefaultClient.Do(reqPlanner)
	require.NoError(t, err)
	defer respPlanner.Body.Close()
	require.Contains(t, []int{http.StatusOK, http.StatusAccepted}, respPlanner.StatusCode)

	require.Eventually(t, func() bool {
		snapshots, listErr := turnStore.List(context.Background(), chatstore.TurnQuery{
			ConvID: convID,
			Phase:  "final",
			Limit:  20,
		})
		if listErr != nil {
			return false
		}
		turnIDs := map[string]struct{}{}
		for _, s := range snapshots {
			turnIDs[s.TurnID] = struct{}{}
		}
		return len(turnIDs) >= 2
	}, 6*time.Second, 100*time.Millisecond, "expected two persisted final turns after runtime switch")

	finalSnapshots, err := turnStore.List(context.Background(), chatstore.TurnQuery{
		ConvID: convID,
		Phase:  "final",
		Limit:  20,
	})
	require.NoError(t, err)
	require.NotEmpty(t, finalSnapshots)

	seenInventory := false
	seenPlanner := false
	for _, snapshot := range finalSnapshots {
		switch strings.TrimSpace(snapshot.RuntimeKey) {
		case "inventory@v0":
			seenInventory = true
		case "planner@v1":
			seenPlanner = true
		}
	}
	require.True(t, seenInventory, "expected at least one final turn with inventory runtime")
	require.True(t, seenPlanner, "expected at least one final turn with planner runtime")

	currentRuntime := ""
	require.Eventually(t, func() bool {
		resp, err := http.Get(srv.URL + integrationDebugConversationsPath() + "/" + convID)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return false
		}
		if v, ok := payload["current_runtime_key"].(string); ok && strings.TrimSpace(v) != "" {
			currentRuntime = strings.TrimSpace(v)
		} else if v, ok := payload["runtime_key"].(string); ok {
			currentRuntime = strings.TrimSpace(v)
		}
		return strings.HasPrefix(currentRuntime, "planner")
	}, 6*time.Second, 100*time.Millisecond, "expected conversation current runtime to converge to planner profile")
}

func TestProfileE2E_CreateProfile_AppearsInList_UsableImmediately(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	createResp, err := http.Post(srv.URL+integrationProfilesPath(), "application/json", strings.NewReader(`{
		"slug":"planner",
		"display_name":"Planner",
		"description":"Plans replenishment and triage",
		"runtime":{"system_prompt":"You are a planning assistant.","tools":["inventory.list","inventory.search"]}
	}`))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	listResp, err := http.Get(srv.URL + integrationProfilesPath())
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	require.True(t, hasProfileSlug(listed, "planner"), "expected planner profile in list")

	const convID = "conv-profile-create-1"
	chatResp, err := http.Post(
		srv.URL+integrationChatPath(),
		"application/json",
		strings.NewReader(`{"prompt":"run planner","conv_id":"`+convID+`","runtime_key":"planner"}`),
	)
	require.NoError(t, err)
	defer chatResp.Body.Close()
	require.Equal(t, http.StatusOK, chatResp.StatusCode)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + integrationWSPath() + "?conv_id=" + convID + "&runtime_key=planner"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))
	require.Equal(t, "planner@v1", integrationSemRuntimeKey(helloFrame))
}

func TestProfileE2E_UpdateIncrementsVersion_AndRebuildsRuntime(t *testing.T) {
	srv := newIntegrationServerWithRouterOptions(t, webchat.WithDebugRoutesEnabled(true))
	defer srv.Close()

	createResp, err := http.Post(srv.URL+integrationProfilesPath(), "application/json", strings.NewReader(`{
		"slug":"rebuilder",
		"display_name":"Rebuilder",
		"description":"Profile used for runtime rebuild checks",
		"runtime":{"system_prompt":"Version one profile prompt"}
	}`))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created map[string]any
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
	initialVersion := extractProfileVersion(created)
	require.Equal(t, uint64(1), initialVersion)

	const convID = "conv-profile-rebuild-1"
	chatRespV1, err := http.Post(
		srv.URL+integrationChatPath(),
		"application/json",
		strings.NewReader(`{"prompt":"before update","conv_id":"`+convID+`","runtime_key":"rebuilder"}`),
	)
	require.NoError(t, err)
	defer chatRespV1.Body.Close()
	require.Equal(t, http.StatusOK, chatRespV1.StatusCode)

	runtimeKeyV1 := mustConversationRuntimeKey(t, srv, convID)
	require.Equal(t, "rebuilder@v1", runtimeKeyV1)

	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL+integrationProfilePath("rebuilder"), strings.NewReader(`{
		"expected_version":1,
		"runtime":{"system_prompt":"Version two profile prompt"}
	}`))
	require.NoError(t, err)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	require.NoError(t, err)
	defer patchResp.Body.Close()
	require.Equal(t, http.StatusOK, patchResp.StatusCode)

	var patched map[string]any
	require.NoError(t, json.NewDecoder(patchResp.Body).Decode(&patched))
	require.Equal(t, uint64(2), extractProfileVersion(patched))

	chatRespV2, err := http.Post(
		srv.URL+integrationChatPath(),
		"application/json",
		strings.NewReader(`{"prompt":"after update","conv_id":"`+convID+`","runtime_key":"rebuilder"}`),
	)
	require.NoError(t, err)
	defer chatRespV2.Body.Close()
	require.Equal(t, http.StatusOK, chatRespV2.StatusCode)

	runtimeKeyV2 := mustConversationRuntimeKey(t, srv, convID)
	require.Equal(t, "rebuilder@v2", runtimeKeyV2)
}

func TestProfileE2E_ReadOnlyProfileMutationRejectedWithStableError(t *testing.T) {
	srv := newIntegrationServer(t)
	defer srv.Close()

	createResp, err := http.Post(srv.URL+integrationProfilesPath(), "application/json", strings.NewReader(`{
		"slug":"locked",
		"display_name":"Locked",
		"policy":{"read_only":true},
		"runtime":{"system_prompt":"Read only profile"}
	}`))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL+integrationProfilePath("locked"), strings.NewReader(`{
		"display_name":"Unlocked?"
	}`))
	require.NoError(t, err)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	require.NoError(t, err)
	defer patchResp.Body.Close()
	require.Equal(t, http.StatusForbidden, patchResp.StatusCode)
	patchBody, err := io.ReadAll(patchResp.Body)
	require.NoError(t, err)
	require.Contains(t, string(patchBody), "policy violation")

	deleteReq, err := http.NewRequest(http.MethodDelete, srv.URL+integrationProfilePath("locked"), nil)
	require.NoError(t, err)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	require.NoError(t, err)
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusForbidden, deleteResp.StatusCode)
	deleteBody, err := io.ReadAll(deleteResp.Body)
	require.NoError(t, err)
	require.Contains(t, string(deleteBody), "policy violation")
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
	resp, err := http.Post(srv.URL+integrationChatPath(), "application/json", bytes.NewReader(reqBody))
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
		if len(snapshots) == 0 {
			return false
		}
		for _, snapshot := range snapshots {
			if snapshot.Phase == "final" && strings.TrimSpace(snapshot.Payload) != "" {
				return true
			}
		}
		return false
	}, 6*time.Second, 100*time.Millisecond, "expected persisted final turn snapshot")

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

func integrationSemRuntimeKey(frame []byte) string {
	var env struct {
		Event struct {
			Data struct {
				RuntimeKey      string `json:"runtimeKey"`
				RuntimeKeySnake string `json:"runtime_key"`
			} `json:"data"`
		} `json:"event"`
	}
	if err := json.Unmarshal(frame, &env); err != nil {
		return ""
	}
	if env.Event.Data.RuntimeKey != "" {
		return env.Event.Data.RuntimeKey
	}
	return env.Event.Data.RuntimeKeySnake
}

func hasProfileSlug(items []map[string]any, slug string) bool {
	want := strings.TrimSpace(slug)
	for _, item := range items {
		got, _ := item["slug"].(string)
		if strings.TrimSpace(got) == want {
			return true
		}
	}
	return false
}

func assertProfileListItemContract(t *testing.T, item map[string]any) {
	t.Helper()
	require.NotEmpty(t, item["slug"])
	assertAllowedContractKeys(
		t,
		item,
		"slug",
		"display_name",
		"description",
		"default_prompt",
		"extensions",
		"is_default",
		"version",
	)
}

func assertProfileDocumentContract(t *testing.T, doc map[string]any) {
	t.Helper()
	require.NotEmpty(t, doc["registry"])
	require.NotEmpty(t, doc["slug"])
	_, hasDefault := doc["is_default"]
	require.True(t, hasDefault)
	assertAllowedContractKeys(
		t,
		doc,
		"registry",
		"slug",
		"display_name",
		"description",
		"runtime",
		"policy",
		"metadata",
		"extensions",
		"is_default",
	)
}

func assertAllowedContractKeys(t *testing.T, payload map[string]any, allowed ...string) {
	t.Helper()
	allowedSet := map[string]struct{}{}
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range payload {
		if _, ok := allowedSet[key]; !ok {
			t.Fatalf("unexpected profile API contract key: %s", key)
		}
	}
}

func mustProfileCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	require.NotNil(t, resp)
	for _, ck := range resp.Cookies() {
		if strings.TrimSpace(ck.Name) == "chat_profile" && strings.TrimSpace(ck.Value) != "" {
			return ck
		}
	}
	t.Fatalf("expected chat_profile cookie")
	return nil
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

	resp, err := http.Post(baseURL+integrationConfirmPath()+"/api/requests", "application/json", bytes.NewReader(body))
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

func extractProfileVersion(doc map[string]any) uint64 {
	metadata, _ := doc["metadata"].(map[string]any)
	if metadata == nil {
		return 0
	}
	raw, ok := metadata["version"]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case int:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case uint64:
		return v
	default:
		return 0
	}
}

func mustConversationRuntimeKey(t *testing.T, srv *httptest.Server, convID string) string {
	t.Helper()
	resp, err := http.Get(srv.URL + integrationDebugConversationsPath() + "/" + convID)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	runtimeKey, _ := payload["current_runtime_key"].(string)
	if strings.TrimSpace(runtimeKey) == "" {
		runtimeKey, _ = payload["runtime_key"].(string)
	}
	return strings.TrimSpace(runtimeKey)
}
