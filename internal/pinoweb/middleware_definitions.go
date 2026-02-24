package pinoweb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

type inventoryMiddlewareDefinition struct {
	name   string
	schema map[string]any
	build  func(context.Context, middlewarecfg.BuildDeps, any) (middleware.Middleware, error)
}

func (d inventoryMiddlewareDefinition) Name() string {
	return d.name
}

func (d inventoryMiddlewareDefinition) ConfigJSONSchema() map[string]any {
	return cloneStringAnyMap(d.schema)
}

func (d inventoryMiddlewareDefinition) Build(
	ctx context.Context,
	deps middlewarecfg.BuildDeps,
	cfg any,
) (middleware.Middleware, error) {
	if d.build == nil {
		return nil, fmt.Errorf("middleware %q has no build function", strings.TrimSpace(d.name))
	}
	return d.build(ctx, deps, cfg)
}

func newInventoryMiddlewareDefinitionRegistry() (*middlewarecfg.InMemoryDefinitionRegistry, error) {
	registry := middlewarecfg.NewInMemoryDefinitionRegistry()
	definitions := []middlewarecfg.Definition{
		newInventoryArtifactPolicyDefinition(),
		newInventorySuggestionsPolicyDefinition(),
		newInventoryArtifactGeneratorDefinition(),
	}
	for _, def := range definitions {
		if err := registry.RegisterDefinition(def); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func defaultInventoryMiddlewareUses() []gepprofiles.MiddlewareUse {
	return []gepprofiles.MiddlewareUse{
		{Name: hypercardPolicyMiddlewareName, ID: "artifact-policy"},
		{Name: hypercardSuggestionsMiddlewareName, ID: "suggestions-policy"},
	}
}

func newInventoryArtifactPolicyDefinition() middlewarecfg.Definition {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instructions": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
	type input struct {
		Instructions string `json:"instructions,omitempty"`
	}

	return inventoryMiddlewareDefinition{
		name:   hypercardPolicyMiddlewareName,
		schema: schema,
		build: func(_ context.Context, _ middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
			var in input
			if err := decodeInventoryMiddlewareConfig(cfg, &in); err != nil {
				return nil, err
			}
			return NewInventoryArtifactPolicyMiddleware(InventoryArtifactPolicyConfig{
				Instructions: strings.TrimSpace(in.Instructions),
			}), nil
		},
	}
}

func newInventorySuggestionsPolicyDefinition() middlewarecfg.Definition {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"instructions": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
	type input struct {
		Instructions string `json:"instructions,omitempty"`
	}

	return inventoryMiddlewareDefinition{
		name:   hypercardSuggestionsMiddlewareName,
		schema: schema,
		build: func(_ context.Context, _ middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
			var in input
			if err := decodeInventoryMiddlewareConfig(cfg, &in); err != nil {
				return nil, err
			}
			return NewInventorySuggestionsPolicyMiddleware(InventorySuggestionsPolicyConfig{
				Instructions: strings.TrimSpace(in.Instructions),
			}), nil
		},
	}
}

func newInventoryArtifactGeneratorDefinition() middlewarecfg.Definition {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"require_widget": map[string]any{"type": "boolean"},
			"require_card":   map[string]any{"type": "boolean"},
		},
		"additionalProperties": false,
	}
	type input struct {
		RequireWidget *bool `json:"require_widget,omitempty"`
		RequireCard   *bool `json:"require_card,omitempty"`
	}

	return inventoryMiddlewareDefinition{
		name:   hypercardGeneratorMiddlewareName,
		schema: schema,
		build: func(_ context.Context, _ middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
			var in input
			if err := decodeInventoryMiddlewareConfig(cfg, &in); err != nil {
				return nil, err
			}
			out := InventoryArtifactGeneratorConfig{}
			if in.RequireWidget != nil {
				out.RequireWidget = *in.RequireWidget
			}
			if in.RequireCard != nil {
				out.RequireCard = *in.RequireCard
			}
			return NewInventoryArtifactGeneratorMiddleware(out), nil
		},
	}
}

func decodeInventoryMiddlewareConfig(cfg any, out any) error {
	if cfg == nil || out == nil {
		return nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize middleware config: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode middleware config: %w", err)
	}
	return nil
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneStringAnyMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}
