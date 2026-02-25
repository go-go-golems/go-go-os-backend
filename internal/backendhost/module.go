package backendhost

import (
	"context"
	"net/http"
)

// AppBackendManifest describes one backend-capable launcher app module.
type AppBackendManifest struct {
	AppID        string   `json:"app_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Required     bool     `json:"required,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// AppBackendModule is the backend-side contract paired with a launcher app module.
type AppBackendModule interface {
	Manifest() AppBackendManifest
	MountRoutes(mux *http.ServeMux) error
	Init(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) error
}
