package backendhost

import (
	"context"
	"net/http"

	"github.com/go-go-golems/go-go-os-backend/pkg/docmw"
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

// ReflectiveAppBackendModule is an optional interface for modules that expose
// discoverable API/schema/docs metadata.
type ReflectiveAppBackendModule interface {
	Reflection(ctx context.Context) (*ModuleReflectionDocument, error)
}

// DocumentableAppBackendModule is an optional interface for modules that expose
// frontmatter-backed module documentation pages.
type DocumentableAppBackendModule interface {
	DocStore() *docmw.DocStore
}

type ModuleReflectionDocument struct {
	AppID        string                 `json:"app_id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Capabilities []ReflectionCapability `json:"capabilities,omitempty"`
	Docs         []ReflectionDocLink    `json:"docs,omitempty"`
	APIs         []ReflectionAPI        `json:"apis,omitempty"`
	Schemas      []ReflectionSchemaRef  `json:"schemas,omitempty"`
}

type ReflectionCapability struct {
	ID          string `json:"id"`
	Stability   string `json:"stability,omitempty"`
	Description string `json:"description,omitempty"`
}

type ReflectionDocLink struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
}

type ReflectionAPI struct {
	ID             string   `json:"id"`
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	Summary        string   `json:"summary,omitempty"`
	RequestSchema  string   `json:"request_schema,omitempty"`
	ResponseSchema string   `json:"response_schema,omitempty"`
	ErrorSchema    string   `json:"error_schema,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

type ReflectionSchemaRef struct {
	ID       string `json:"id"`
	Format   string `json:"format"`
	URI      string `json:"uri,omitempty"`
	Embedded any    `json:"embedded,omitempty"`
}
