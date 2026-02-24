package pinoweb

import (
	"context"
	"strings"
	"testing"

	embeddingscfg "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type runtimeComposerTestSection struct {
	slug string
}

func (s runtimeComposerTestSection) GetDefinitions() *fields.Definitions {
	return fields.NewDefinitions()
}
func (s runtimeComposerTestSection) GetName() string        { return s.slug }
func (s runtimeComposerTestSection) GetDescription() string { return "" }
func (s runtimeComposerTestSection) GetPrefix() string      { return "" }
func (s runtimeComposerTestSection) GetSlug() string        { return s.slug }

type runtimeComposerTestDefinition struct {
	name    string
	schema  map[string]any
	buildFn func(context.Context, middlewarecfg.BuildDeps, any) (middleware.Middleware, error)
}

func (d *runtimeComposerTestDefinition) Name() string {
	return d.name
}

func (d *runtimeComposerTestDefinition) ConfigJSONSchema() map[string]any {
	return d.schema
}

func (d *runtimeComposerTestDefinition) Build(
	ctx context.Context,
	deps middlewarecfg.BuildDeps,
	cfg any,
) (middleware.Middleware, error) {
	if d.buildFn == nil {
		return func(next middleware.HandlerFunc) middleware.HandlerFunc { return next }, nil
	}
	return d.buildFn(ctx, deps, cfg)
}

func minimalRuntimeComposerValues(t *testing.T) *values.Values {
	t.Helper()

	slugs := []string{
		settings.AiClientSlug,
		settings.AiChatSlug,
		openai.OpenAiChatSlug,
		claude.ClaudeChatSlug,
		gemini.GeminiChatSlug,
		embeddingscfg.EmbeddingsSlug,
		settings.AiInferenceSlug,
	}
	opts := make([]values.ValuesOption, 0, len(slugs))
	for _, slug := range slugs {
		sectionValues, err := values.NewSectionValues(runtimeComposerTestSection{slug: slug})
		if err != nil {
			t.Fatalf("new section values for %s: %v", slug, err)
		}
		if slug == openai.OpenAiChatSlug {
			sectionValues.Fields.Update("openai-api-key", &fields.FieldValue{Value: "test-api-key"})
		}
		opts = append(opts, values.WithSectionValues(slug, sectionValues))
	}
	return values.New(opts...)
}

func newRuntimeComposerDefinitionRegistry(t *testing.T, defs ...middlewarecfg.Definition) middlewarecfg.DefinitionRegistry {
	t.Helper()

	registry := middlewarecfg.NewInMemoryDefinitionRegistry()
	for _, def := range defs {
		if err := registry.RegisterDefinition(def); err != nil {
			t.Fatalf("register middleware definition %q: %v", def.Name(), err)
		}
	}
	return registry
}

func TestRuntimeComposer_AppliesProfileMiddlewaresWithResolverCoercion(t *testing.T) {
	var builtConfig map[string]any
	def := &runtimeComposerTestDefinition{
		name: "inventory_artifact_policy",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"threshold": map[string]any{"type": "integer"},
				"mode":      map[string]any{"type": "string"},
			},
		},
		buildFn: func(_ context.Context, _ middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
			builtConfig, _ = cfg.(map[string]any)
			return func(next middleware.HandlerFunc) middleware.HandlerFunc { return next }, nil
		},
	}

	composer := newRuntimeComposerWithDefinitions(
		minimalRuntimeComposerValues(t),
		RuntimeComposerOptions{RuntimeKey: "inventory"},
		newRuntimeComposerDefinitionRegistry(t, def),
		middlewarecfg.BuildDeps{},
		nil,
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ProfileKey: "inventory",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
			Middlewares: []gepprofiles.MiddlewareUse{
				{
					Name: "inventory_artifact_policy",
					ID:   "policy",
					Config: map[string]any{
						"threshold": "7",
						"mode":      "strict",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if builtConfig == nil {
		t.Fatalf("expected middleware to be built")
	}
	if got, want := builtConfig["threshold"], int64(7); got != want {
		t.Fatalf("threshold coercion mismatch: got=%#v want=%#v", got, want)
	}
	if got, want := builtConfig["mode"], "strict"; got != want {
		t.Fatalf("mode mismatch: got=%#v want=%#v", got, want)
	}
}

func TestRuntimeComposer_ExplicitEmptyProfileMiddlewaresDoNotFallback(t *testing.T) {
	buildCalls := 0
	def := &runtimeComposerTestDefinition{
		name: "inventory_artifact_policy",
		schema: map[string]any{
			"type": "object",
		},
		buildFn: func(_ context.Context, _ middlewarecfg.BuildDeps, _ any) (middleware.Middleware, error) {
			buildCalls++
			return func(next middleware.HandlerFunc) middleware.HandlerFunc { return next }, nil
		},
	}

	composer := newRuntimeComposerWithDefinitions(
		minimalRuntimeComposerValues(t),
		RuntimeComposerOptions{RuntimeKey: "inventory"},
		newRuntimeComposerDefinitionRegistry(t, def),
		middlewarecfg.BuildDeps{},
		[]gepprofiles.MiddlewareUse{
			{Name: "inventory_artifact_policy", ID: "default"},
		},
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ProfileKey: "default",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
			Middlewares: []gepprofiles.MiddlewareUse{},
		},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if buildCalls != 0 {
		t.Fatalf("expected no middleware builds for explicit empty profile middlewares, got %d", buildCalls)
	}
}

func TestRuntimeComposer_RejectsInvalidMiddlewareSchemaPayload(t *testing.T) {
	def := &runtimeComposerTestDefinition{
		name: "inventory_artifact_policy",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"threshold": map[string]any{"type": "integer"},
			},
		},
	}

	composer := newRuntimeComposerWithDefinitions(
		minimalRuntimeComposerValues(t),
		RuntimeComposerOptions{RuntimeKey: "inventory"},
		newRuntimeComposerDefinitionRegistry(t, def),
		middlewarecfg.BuildDeps{},
		nil,
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ProfileKey: "inventory",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
			Middlewares: []gepprofiles.MiddlewareUse{
				{
					Name: "inventory_artifact_policy",
					Config: map[string]any{
						"threshold": "not-a-number",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected schema validation error")
	}
	if !strings.Contains(err.Error(), "/threshold") {
		t.Fatalf("expected schema path context, got: %v", err)
	}
	if !strings.Contains(err.Error(), "resolve middleware") {
		t.Fatalf("expected middleware resolution context, got: %v", err)
	}
}
