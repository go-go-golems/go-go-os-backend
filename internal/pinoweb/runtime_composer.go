package pinoweb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gepmw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/pkg/errors"
)

type RuntimeComposerOptions struct {
	RuntimeKey   string
	SystemPrompt string
	AllowedTools []string
}

type RuntimeComposer struct {
	parsed             *values.Values
	options            RuntimeComposerOptions
	definitions        middlewarecfg.DefinitionRegistry
	buildDeps          middlewarecfg.BuildDeps
	defaultMiddlewares []gepprofiles.MiddlewareUse
}

func NewRuntimeComposer(parsed *values.Values, options RuntimeComposerOptions) *RuntimeComposer {
	registry, err := newInventoryMiddlewareDefinitionRegistry()
	if err != nil {
		panic(fmt.Sprintf("pinoweb: build middleware definitions: %v", err))
	}
	return newRuntimeComposerWithDefinitions(
		parsed,
		options,
		registry,
		middlewarecfg.BuildDeps{},
		defaultInventoryMiddlewareUses(),
	)
}

func newRuntimeComposerWithDefinitions(
	parsed *values.Values,
	options RuntimeComposerOptions,
	definitions middlewarecfg.DefinitionRegistry,
	buildDeps middlewarecfg.BuildDeps,
	defaultMiddlewares []gepprofiles.MiddlewareUse,
) *RuntimeComposer {
	return &RuntimeComposer{
		parsed:             parsed,
		options:            options,
		definitions:        definitions,
		buildDeps:          buildDeps,
		defaultMiddlewares: cloneMiddlewareUses(defaultMiddlewares),
	}
}

func (c *RuntimeComposer) MiddlewareDefinitions() middlewarecfg.DefinitionRegistry {
	if c == nil {
		return nil
	}
	return c.definitions
}

func (c *RuntimeComposer) Compose(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
	if c == nil || c.parsed == nil {
		return infruntime.ComposedRuntime{}, errors.New("runtime composer is not configured")
	}
	if ctx == nil {
		return infruntime.ComposedRuntime{}, errors.New("compose context is nil")
	}

	profileRuntime := req.ResolvedProfileRuntime

	stepSettings, err := settings.NewStepSettingsFromParsedValues(c.parsed)
	if err != nil {
		return infruntime.ComposedRuntime{}, errors.Wrap(err, "parse step settings")
	}
	effectiveStepSettings := stepSettings.Clone()
	if profileRuntime != nil && len(profileRuntime.StepSettingsPatch) > 0 {
		effectiveStepSettings, err = gepprofiles.ApplyRuntimeStepSettingsPatch(stepSettings, profileRuntime.StepSettingsPatch)
		if err != nil {
			return infruntime.ComposedRuntime{}, errors.Wrap(err, "apply profile step_settings_patch")
		}
	}

	runtimeKey := strings.TrimSpace(req.ProfileKey)
	if runtimeKey == "" {
		runtimeKey = strings.TrimSpace(c.options.RuntimeKey)
	}
	if runtimeKey == "" {
		runtimeKey = "inventory"
	}

	systemPrompt := strings.TrimSpace(c.options.SystemPrompt)
	if profileRuntime != nil && strings.TrimSpace(profileRuntime.SystemPrompt) != "" {
		systemPrompt = strings.TrimSpace(profileRuntime.SystemPrompt)
	}
	if systemPrompt == "" {
		systemPrompt = "You are an inventory assistant."
	}
	allowedTools := append([]string(nil), c.options.AllowedTools...)
	if profileRuntime != nil && len(profileRuntime.Tools) > 0 {
		allowedTools = append([]string(nil), profileRuntime.Tools...)
	}

	profileMiddlewares, err := runtimeMiddlewaresFromProfile(profileRuntime)
	if err != nil {
		return infruntime.ComposedRuntime{}, errors.Wrap(err, "normalize profile middlewares")
	}
	if profileRuntime == nil {
		profileMiddlewares = cloneMiddlewareUses(c.defaultMiddlewares)
	}

	resolvedMiddlewares, resolvedUses, err := c.resolveMiddlewares(ctx, profileMiddlewares)
	if err != nil {
		return infruntime.ComposedRuntime{}, errors.Wrap(err, "resolve middlewares")
	}

	engine_, err := infruntime.BuildEngineFromSettingsWithMiddlewares(
		ctx,
		effectiveStepSettings,
		systemPrompt,
		resolvedMiddlewares,
	)
	if err != nil {
		return infruntime.ComposedRuntime{}, errors.Wrap(err, "compose engine")
	}

	return infruntime.ComposedRuntime{
		Engine:     engine_,
		RuntimeKey: runtimeKey,
		RuntimeFingerprint: runtimeFingerprint(runtimeFingerprintInput{
			ProfileVersion:      req.ProfileVersion,
			RuntimeKey:          runtimeKey,
			SystemPrompt:        systemPrompt,
			AllowedTools:        allowedTools,
			ResolvedMiddlewares: resolvedUses,
			ProfileRuntime:      profileRuntime,
			StepSettings:        effectiveStepSettings,
		}),
		SeedSystemPrompt: systemPrompt,
		AllowedTools:     allowedTools,
	}, nil
}

func (c *RuntimeComposer) resolveMiddlewares(
	ctx context.Context,
	profileMiddlewares []gepprofiles.MiddlewareUse,
) ([]gepmw.Middleware, []gepprofiles.MiddlewareUse, error) {
	if len(profileMiddlewares) == 0 {
		return nil, nil, nil
	}
	if c == nil || c.definitions == nil {
		return nil, nil, errors.New("middleware definitions are not configured")
	}

	resolvedInstances := make([]middlewarecfg.ResolvedInstance, 0, len(profileMiddlewares))
	resolvedUses := make([]gepprofiles.MiddlewareUse, 0, len(profileMiddlewares))
	for i, use := range profileMiddlewares {
		instanceKey := middlewarecfg.MiddlewareInstanceKey(use, i)
		def, ok := c.definitions.GetDefinition(use.Name)
		if !ok {
			return nil, nil, fmt.Errorf("unknown middleware %s", instanceKey)
		}

		sourcePayload, err := normalizeConfigObject(use.Config, fmt.Sprintf("profile middleware %s config", instanceKey))
		if err != nil {
			return nil, nil, err
		}
		sources := make([]middlewarecfg.Source, 0, 1)
		if len(sourcePayload) > 0 {
			sources = append(sources, fixedPayloadSource{
				name:    "profile",
				layer:   middlewarecfg.SourceLayerProfile,
				payload: sourcePayload,
			})
		}

		resolver := middlewarecfg.NewResolver(sources...)
		resolvedCfg, err := resolver.Resolve(def, gepprofiles.MiddlewareUse{
			Name:    use.Name,
			ID:      use.ID,
			Enabled: cloneBoolPtr(use.Enabled),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("resolve middleware %s: %w", instanceKey, err)
		}

		resolvedInstances = append(resolvedInstances, middlewarecfg.ResolvedInstance{
			Key:      instanceKey,
			Use:      use,
			Resolved: resolvedCfg,
			Def:      def,
		})
		resolvedUses = append(resolvedUses, gepprofiles.MiddlewareUse{
			Name:    use.Name,
			ID:      use.ID,
			Enabled: cloneBoolPtr(use.Enabled),
			Config:  cloneStringAnyMap(resolvedCfg.Config),
		})
	}

	chain, err := middlewarecfg.BuildChain(ctx, c.buildDeps, resolvedInstances)
	if err != nil {
		return nil, nil, err
	}
	return chain, resolvedUses, nil
}

type fixedPayloadSource struct {
	name    string
	layer   middlewarecfg.SourceLayer
	payload map[string]any
}

func (s fixedPayloadSource) Name() string {
	return s.name
}

func (s fixedPayloadSource) Layer() middlewarecfg.SourceLayer {
	return s.layer
}

func (s fixedPayloadSource) Payload(middlewarecfg.Definition, gepprofiles.MiddlewareUse) (map[string]any, bool, error) {
	if len(s.payload) == 0 {
		return nil, false, nil
	}
	return cloneStringAnyMap(s.payload), true, nil
}

func runtimeMiddlewaresFromProfile(spec *gepprofiles.RuntimeSpec) ([]gepprofiles.MiddlewareUse, error) {
	if spec == nil || len(spec.Middlewares) == 0 {
		return nil, nil
	}
	middlewares := make([]gepprofiles.MiddlewareUse, 0, len(spec.Middlewares))
	for i, mw := range spec.Middlewares {
		name := strings.TrimSpace(mw.Name)
		if name == "" {
			continue
		}
		config, err := normalizeConfigObject(mw.Config, fmt.Sprintf("profile middleware %s config", middlewarecfg.MiddlewareInstanceKey(mw, i)))
		if err != nil {
			return nil, err
		}
		middlewares = append(middlewares, gepprofiles.MiddlewareUse{
			Name:    name,
			ID:      strings.TrimSpace(mw.ID),
			Enabled: cloneBoolPtr(mw.Enabled),
			Config:  config,
		})
	}
	if len(middlewares) == 0 {
		return nil, nil
	}
	return middlewares, nil
}

func cloneMiddlewareUses(in []gepprofiles.MiddlewareUse) []gepprofiles.MiddlewareUse {
	if len(in) == 0 {
		return nil
	}
	out := make([]gepprofiles.MiddlewareUse, 0, len(in))
	for _, mw := range in {
		name := strings.TrimSpace(mw.Name)
		if name == "" {
			continue
		}
		config, _ := normalizeConfigObject(mw.Config, "")
		out = append(out, gepprofiles.MiddlewareUse{
			Name:    name,
			ID:      strings.TrimSpace(mw.ID),
			Enabled: cloneBoolPtr(mw.Enabled),
			Config:  config,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

type runtimeFingerprintInput struct {
	ProfileVersion      uint64
	RuntimeKey          string
	SystemPrompt        string
	AllowedTools        []string
	ResolvedMiddlewares []gepprofiles.MiddlewareUse
	ProfileRuntime      *gepprofiles.RuntimeSpec
	StepSettings        *settings.StepSettings
}

func runtimeFingerprint(in runtimeFingerprintInput) string {
	payload := map[string]any{
		"profile_version":      in.ProfileVersion,
		"runtime_key":          in.RuntimeKey,
		"system_prompt":        in.SystemPrompt,
		"allowed_tools":        in.AllowedTools,
		"resolved_middlewares": in.ResolvedMiddlewares,
	}
	if in.ProfileRuntime != nil {
		payload["profile_runtime"] = map[string]any{
			"system_prompt":       in.ProfileRuntime.SystemPrompt,
			"middlewares":         in.ProfileRuntime.Middlewares,
			"tools":               in.ProfileRuntime.Tools,
			"step_settings_patch": in.ProfileRuntime.StepSettingsPatch,
		}
	}
	if in.StepSettings != nil {
		payload["step_metadata"] = in.StepSettings.GetMetadata()
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return in.RuntimeKey
	}
	return string(b)
}

func normalizeConfigObject(raw any, context string) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	if object, ok := raw.(map[string]any); ok {
		return cloneStringAnyMap(object), nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be JSON-serializable: %w", strings.TrimSpace(context), err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("%s must be an object: %w", strings.TrimSpace(context), err)
	}
	return out, nil
}
