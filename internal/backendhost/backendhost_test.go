package backendhost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeModule struct {
	manifest  AppBackendManifest
	mountFn   func(mux *http.ServeMux) error
	initErr   error
	startErr  error
	stopErr   error
	healthErr error

	reflectionDoc *ModuleReflectionDocument
	reflectionErr error
}

type fakePlainModule struct {
	manifest  AppBackendManifest
	mountFn   func(mux *http.ServeMux) error
	initErr   error
	startErr  error
	stopErr   error
	healthErr error
}

func (f *fakeModule) Manifest() AppBackendManifest {
	return f.manifest
}

func (f *fakeModule) MountRoutes(mux *http.ServeMux) error {
	if f.mountFn != nil {
		return f.mountFn(mux)
	}
	return nil
}

func (f *fakeModule) Init(context.Context) error {
	return f.initErr
}

func (f *fakeModule) Start(context.Context) error {
	return f.startErr
}

func (f *fakeModule) Stop(context.Context) error {
	return f.stopErr
}

func (f *fakeModule) Health(context.Context) error {
	return f.healthErr
}

func (f *fakeModule) Reflection(context.Context) (*ModuleReflectionDocument, error) {
	return f.reflectionDoc, f.reflectionErr
}

func (f *fakePlainModule) Manifest() AppBackendManifest {
	return f.manifest
}

func (f *fakePlainModule) MountRoutes(mux *http.ServeMux) error {
	if f.mountFn != nil {
		return f.mountFn(mux)
	}
	return nil
}

func (f *fakePlainModule) Init(context.Context) error {
	return f.initErr
}

func (f *fakePlainModule) Start(context.Context) error {
	return f.startErr
}

func (f *fakePlainModule) Stop(context.Context) error {
	return f.stopErr
}

func (f *fakePlainModule) Health(context.Context) error {
	return f.healthErr
}

func TestNewModuleRegistry_RejectsDuplicateAppID(t *testing.T) {
	_, err := NewModuleRegistry(
		&fakeModule{manifest: AppBackendManifest{AppID: "inventory"}},
		&fakeModule{manifest: AppBackendManifest{AppID: "inventory"}},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate backend module app id")
}

func TestMountNamespacedRoutes_MountsUnderAppPrefix(t *testing.T) {
	mux := http.NewServeMux()
	err := MountNamespacedRoutes(mux, "inventory", func(sub *http.ServeMux) error {
		sub.HandleFunc("/chat", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})
		return nil
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/apps/inventory/chat", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "ok", rr.Body.String())
}

func TestGuardNoLegacyAliases_FailsForForbiddenPaths(t *testing.T) {
	err := GuardNoLegacyAliases([]string{"/chat", "/api/apps/inventory/chat"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "legacy route alias")
}

func TestLifecycleManager_RequiredModuleMissingFails(t *testing.T) {
	registry, err := NewModuleRegistry(&fakeModule{manifest: AppBackendManifest{AppID: "inventory"}})
	require.NoError(t, err)

	manager := NewLifecycleManager(registry)
	err = manager.Startup(context.Background(), StartupOptions{RequiredAppIDs: []string{"inventory", "crm"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required backend module \"crm\" is not registered")
}

func TestLifecycleManager_RequiredHealthFailureFailsStartup(t *testing.T) {
	registry, err := NewModuleRegistry(
		&fakeModule{manifest: AppBackendManifest{AppID: "inventory", Required: true}, healthErr: errors.New("down")},
	)
	require.NoError(t, err)

	manager := NewLifecycleManager(registry)
	err = manager.Startup(context.Background(), StartupOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "health check failed")
}

func TestRegisterAppsManifestEndpoint_ReturnsManifestAndHealth(t *testing.T) {
	registry, err := NewModuleRegistry(
		&fakeModule{
			manifest: AppBackendManifest{AppID: "inventory", Name: "Inventory", Required: true, Capabilities: []string{"chat"}},
			reflectionDoc: &ModuleReflectionDocument{
				AppID: "inventory",
				Name:  "Inventory",
			},
		},
		&fakePlainModule{manifest: AppBackendManifest{AppID: "crm", Name: "CRM"}, healthErr: errors.New("offline")},
	)
	require.NoError(t, err)

	mux := http.NewServeMux()
	RegisterAppsManifestEndpoint(mux, registry)

	req := httptest.NewRequest(http.MethodGet, "/api/os/apps", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var payload AppManifestResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&payload))
	require.Len(t, payload.Apps, 2)
	require.Equal(t, "inventory", payload.Apps[0].AppID)
	require.True(t, payload.Apps[0].Healthy)
	require.NotNil(t, payload.Apps[0].Reflection)
	require.Equal(t, "/api/os/apps/inventory/reflection", payload.Apps[0].Reflection.URL)
	require.Equal(t, "crm", payload.Apps[1].AppID)
	require.False(t, payload.Apps[1].Healthy)
	require.Equal(t, "offline", payload.Apps[1].HealthError)
	require.Nil(t, payload.Apps[1].Reflection)
}

func TestRegisterAppsManifestEndpoint_ModuleReflectionRoute_ReturnsPayload(t *testing.T) {
	registry, err := NewModuleRegistry(
		&fakeModule{
			manifest: AppBackendManifest{AppID: "inventory", Name: "Inventory"},
			reflectionDoc: &ModuleReflectionDocument{
				AppID:   "inventory",
				Name:    "Inventory",
				Summary: "inventory reflection",
				APIs: []ReflectionAPI{
					{
						ID:     "list",
						Method: http.MethodGet,
						Path:   "/api/apps/inventory/chat",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	mux := http.NewServeMux()
	RegisterAppsManifestEndpoint(mux, registry)

	req := httptest.NewRequest(http.MethodGet, "/api/os/apps/inventory/reflection", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var payload ModuleReflectionDocument
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&payload))
	require.Equal(t, "inventory", payload.AppID)
	require.Equal(t, "Inventory", payload.Name)
	require.Len(t, payload.APIs, 1)
	require.Equal(t, "/api/apps/inventory/chat", payload.APIs[0].Path)
}

func TestRegisterAppsManifestEndpoint_ModuleReflectionRoute_NotImplemented(t *testing.T) {
	registry, err := NewModuleRegistry(
		&fakePlainModule{
			manifest: AppBackendManifest{AppID: "inventory", Name: "Inventory"},
		},
	)
	require.NoError(t, err)

	mux := http.NewServeMux()
	RegisterAppsManifestEndpoint(mux, registry)

	req := httptest.NewRequest(http.MethodGet, "/api/os/apps/inventory/reflection", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotImplemented, rr.Code)

	reqUnknown := httptest.NewRequest(http.MethodGet, "/api/os/apps/unknown/reflection", nil)
	rrUnknown := httptest.NewRecorder()
	mux.ServeHTTP(rrUnknown, reqUnknown)
	require.Equal(t, http.StatusNotFound, rrUnknown.Code)
}
