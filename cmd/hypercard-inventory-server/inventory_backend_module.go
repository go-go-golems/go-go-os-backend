package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	plzconfirmbackend "github.com/go-go-golems/plz-confirm/pkg/backend"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/backendhost"
)

const inventoryBackendAppID = "inventory"

type inventoryBackendModule struct {
	server                *webchat.Server
	requestResolver       webhttp.ConversationRequestResolver
	profileRegistry       gepprofiles.Registry
	middlewareDefinitions middlewarecfg.DefinitionRegistry
	extensionSchemas      []webhttp.ExtensionSchemaDocument
}

func newInventoryBackendModule(
	server *webchat.Server,
	requestResolver webhttp.ConversationRequestResolver,
	profileRegistry gepprofiles.Registry,
	middlewareDefinitions middlewarecfg.DefinitionRegistry,
	extensionSchemas []webhttp.ExtensionSchemaDocument,
) *inventoryBackendModule {
	return &inventoryBackendModule{
		server:                server,
		requestResolver:       requestResolver,
		profileRegistry:       profileRegistry,
		middlewareDefinitions: middlewareDefinitions,
		extensionSchemas:      append([]webhttp.ExtensionSchemaDocument(nil), extensionSchemas...),
	}
}

func (m *inventoryBackendModule) Manifest() backendhost.AppBackendManifest {
	return backendhost.AppBackendManifest{
		AppID:       inventoryBackendAppID,
		Name:        "Inventory",
		Description: "Inventory chat runtime, profiles, timeline, and confirm APIs",
		Required:    true,
		Capabilities: []string{
			"chat",
			"ws",
			"timeline",
			"profiles",
			"confirm",
		},
	}
}

func (m *inventoryBackendModule) MountRoutes(mux *http.ServeMux) error {
	if mux == nil {
		return fmt.Errorf("inventory backend module mount mux is nil")
	}
	if m.server == nil {
		return fmt.Errorf("inventory backend module server is nil")
	}
	if m.requestResolver == nil {
		return fmt.Errorf("inventory backend module request resolver is nil")
	}
	if m.profileRegistry == nil {
		return fmt.Errorf("inventory backend module profile registry is nil")
	}

	chatHandler := webhttp.NewChatHandler(m.server.ChatService(), m.requestResolver)
	wsHandler := webhttp.NewWSHandler(
		m.server.StreamHub(),
		m.requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)
	timelineHandler := webhttp.NewTimelineHandler(
		m.server.TimelineService(),
		log.With().Str("component", "inventory-chat").Str("route", "/api/apps/inventory/api/timeline").Logger(),
	)

	mux.HandleFunc("/chat", chatHandler)
	mux.HandleFunc("/chat/", chatHandler)
	mux.HandleFunc("/ws", wsHandler)
	webhttp.RegisterProfileAPIHandlers(mux, m.profileRegistry, webhttp.ProfileAPIHandlerOptions{
		DefaultRegistrySlug:             gepprofiles.MustRegistrySlug(profileRegistrySlug),
		EnableCurrentProfileCookieRoute: true,
		WriteActor:                      "hypercard-inventory-server",
		WriteSource:                     "http-api",
		MiddlewareDefinitions:           m.middlewareDefinitions,
		ExtensionSchemas:                append([]webhttp.ExtensionSchemaDocument(nil), m.extensionSchemas...),
	})
	mux.HandleFunc("/api/timeline", timelineHandler)
	mux.HandleFunc("/api/timeline/", timelineHandler)
	mux.Handle("/api/", m.server.APIHandler())
	plzconfirmbackend.NewServer().Mount(mux, "/confirm")
	mux.Handle("/", m.server.UIHandler())

	return nil
}

func (m *inventoryBackendModule) Init(context.Context) error {
	if m == nil || m.server == nil {
		return fmt.Errorf("inventory backend module is not initialized")
	}
	return nil
}

func (m *inventoryBackendModule) Start(context.Context) error {
	if m == nil || m.server == nil {
		return fmt.Errorf("inventory backend module is not initialized")
	}
	return nil
}

func (m *inventoryBackendModule) Stop(context.Context) error {
	return nil
}

func (m *inventoryBackendModule) Health(context.Context) error {
	if m == nil || m.server == nil {
		return fmt.Errorf("inventory backend module server is not initialized")
	}
	return nil
}
