package backendhost

import (
	"fmt"
	"strings"
)

// ModuleRegistry stores backend modules keyed by app id.
type ModuleRegistry struct {
	ordered []AppBackendModule
	byAppID map[string]AppBackendModule
}

func NewModuleRegistry(modules ...AppBackendModule) (*ModuleRegistry, error) {
	registry := &ModuleRegistry{
		ordered: make([]AppBackendModule, 0, len(modules)),
		byAppID: map[string]AppBackendModule{},
	}

	for _, module := range modules {
		if module == nil {
			return nil, fmt.Errorf("backend module is nil")
		}
		manifest := module.Manifest()
		appID := strings.TrimSpace(manifest.AppID)
		if err := ValidateAppID(appID); err != nil {
			return nil, fmt.Errorf("invalid backend module app id %q: %w", manifest.AppID, err)
		}
		if _, ok := registry.byAppID[appID]; ok {
			return nil, fmt.Errorf("duplicate backend module app id %q", appID)
		}
		manifest.AppID = appID
		registry.ordered = append(registry.ordered, module)
		registry.byAppID[appID] = module
	}

	return registry, nil
}

func (r *ModuleRegistry) Modules() []AppBackendModule {
	if r == nil || len(r.ordered) == 0 {
		return nil
	}
	out := make([]AppBackendModule, len(r.ordered))
	copy(out, r.ordered)
	return out
}

func (r *ModuleRegistry) Get(appID string) (AppBackendModule, bool) {
	if r == nil {
		return nil, false
	}
	module, ok := r.byAppID[strings.TrimSpace(appID)]
	return module, ok
}

func (r *ModuleRegistry) AppIDs() []string {
	if r == nil || len(r.ordered) == 0 {
		return nil
	}
	ids := make([]string, 0, len(r.ordered))
	for _, module := range r.ordered {
		ids = append(ids, strings.TrimSpace(module.Manifest().AppID))
	}
	return ids
}
