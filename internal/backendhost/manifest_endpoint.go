package backendhost

import (
	"encoding/json"
	"net/http"
	"strings"
)

type AppManifestDocument struct {
	AppID        string   `json:"app_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Required     bool     `json:"required"`
	Capabilities []string `json:"capabilities,omitempty"`
	Healthy      bool     `json:"healthy"`
	HealthError  string   `json:"health_error,omitempty"`
}

type AppManifestResponse struct {
	Apps []AppManifestDocument `json:"apps"`
}

func RegisterAppsManifestEndpoint(mux *http.ServeMux, registry *ModuleRegistry) {
	if mux == nil || registry == nil {
		return
	}

	mux.HandleFunc("/api/os/apps", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := AppManifestResponse{Apps: make([]AppManifestDocument, 0, len(registry.Modules()))}
		for _, module := range registry.Modules() {
			manifest := module.Manifest()
			healthErr := module.Health(req.Context())
			doc := AppManifestDocument{
				AppID:        strings.TrimSpace(manifest.AppID),
				Name:         strings.TrimSpace(manifest.Name),
				Description:  strings.TrimSpace(manifest.Description),
				Required:     manifest.Required,
				Capabilities: append([]string(nil), manifest.Capabilities...),
				Healthy:      healthErr == nil,
			}
			if healthErr != nil {
				doc.HealthError = healthErr.Error()
			}
			response.Apps = append(response.Apps, doc)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
}
