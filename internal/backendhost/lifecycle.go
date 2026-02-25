package backendhost

import (
	"context"
	"fmt"
	"strings"
)

type StartupOptions struct {
	RequiredAppIDs []string
}

// LifecycleManager handles module init/start/health/stop ordering.
type LifecycleManager struct {
	registry *ModuleRegistry
	started  []AppBackendModule
}

func NewLifecycleManager(registry *ModuleRegistry) *LifecycleManager {
	return &LifecycleManager{registry: registry}
}

func (m *LifecycleManager) Startup(ctx context.Context, opts StartupOptions) error {
	if m == nil {
		return fmt.Errorf("lifecycle manager is nil")
	}
	if m.registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	if err := m.validateRequired(opts.RequiredAppIDs); err != nil {
		return err
	}

	modules := m.registry.Modules()
	m.started = nil
	for _, module := range modules {
		manifest := module.Manifest()
		appID := strings.TrimSpace(manifest.AppID)
		if err := module.Init(ctx); err != nil {
			_ = m.Stop(ctx)
			return fmt.Errorf("init module %q: %w", appID, err)
		}
		if err := module.Start(ctx); err != nil {
			_ = m.Stop(ctx)
			return fmt.Errorf("start module %q: %w", appID, err)
		}
		m.started = append(m.started, module)
	}

	requiredSet := m.requiredSet(opts.RequiredAppIDs)
	for _, module := range modules {
		manifest := module.Manifest()
		appID := strings.TrimSpace(manifest.AppID)
		if !requiredSet[appID] {
			continue
		}
		if err := module.Health(ctx); err != nil {
			_ = m.Stop(ctx)
			return fmt.Errorf("health check failed for required module %q: %w", appID, err)
		}
	}

	return nil
}

func (m *LifecycleManager) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	var errs []string
	for i := len(m.started) - 1; i >= 0; i-- {
		module := m.started[i]
		manifest := module.Manifest()
		if err := module.Stop(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", strings.TrimSpace(manifest.AppID), err))
		}
	}
	m.started = nil
	if len(errs) > 0 {
		return fmt.Errorf("module stop failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (m *LifecycleManager) validateRequired(requiredIDs []string) error {
	requiredSet := m.requiredSet(requiredIDs)
	for appID := range requiredSet {
		if _, ok := m.registry.Get(appID); !ok {
			return fmt.Errorf("required backend module %q is not registered", appID)
		}
	}
	return nil
}

func (m *LifecycleManager) requiredSet(requiredIDs []string) map[string]bool {
	required := map[string]bool{}
	for _, appID := range requiredIDs {
		trimmed := strings.TrimSpace(appID)
		if trimmed == "" {
			continue
		}
		required[trimmed] = true
	}
	if m.registry == nil {
		return required
	}
	for _, module := range m.registry.Modules() {
		manifest := module.Manifest()
		if manifest.Required {
			required[strings.TrimSpace(manifest.AppID)] = true
		}
	}
	return required
}
