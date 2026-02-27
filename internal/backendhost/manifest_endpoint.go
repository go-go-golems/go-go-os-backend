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
	Reflection   *AppManifestReflectionHint `json:"reflection,omitempty"`
	Healthy      bool     `json:"healthy"`
	HealthError  string   `json:"health_error,omitempty"`
}

type AppManifestReflectionHint struct {
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
	Version   string `json:"version,omitempty"`
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
			if _, ok := module.(ReflectiveAppBackendModule); ok {
				doc.Reflection = &AppManifestReflectionHint{
					Available: true,
					URL:       "/api/os/apps/" + doc.AppID + "/reflection",
					Version:   "v1",
				}
			}
			if healthErr != nil {
				doc.HealthError = healthErr.Error()
			}
			response.Apps = append(response.Apps, doc)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/api/os/apps/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		appID, ok := parseReflectionPath(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}

		module, exists := registry.Get(appID)
		if !exists {
			http.NotFound(w, req)
			return
		}
		reflective, ok := module.(ReflectiveAppBackendModule)
		if !ok {
			http.Error(w, "reflection not implemented", http.StatusNotImplemented)
			return
		}

		reflection, err := reflective.Reflection(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if reflection == nil {
			http.Error(w, "reflection document is nil", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reflection)
	})
}

func parseReflectionPath(path string) (string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/os/apps/")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[1] != "reflection" {
		return "", false
	}
	appID := strings.TrimSpace(parts[0])
	if appID == "" {
		return "", false
	}
	return appID, true
}
