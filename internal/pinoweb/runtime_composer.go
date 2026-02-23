package pinoweb

import (
	"context"
	"encoding/json"
	"strings"

	gepmw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
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
	parsed  *values.Values
	options RuntimeComposerOptions
}

func NewRuntimeComposer(parsed *values.Values, options RuntimeComposerOptions) *RuntimeComposer {
	return &RuntimeComposer{parsed: parsed, options: options}
}

func (c *RuntimeComposer) Compose(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
	if c == nil || c.parsed == nil {
		return infruntime.ComposedRuntime{}, errors.New("runtime composer is not configured")
	}
	if ctx == nil {
		return infruntime.ComposedRuntime{}, errors.New("compose context is nil")
	}

	if len(req.RuntimeOverrides) > 0 {
		return infruntime.ComposedRuntime{}, errors.New("runtime overrides are not allowed for inventory")
	}

	stepSettings, err := settings.NewStepSettingsFromParsedValues(c.parsed)
	if err != nil {
		return infruntime.ComposedRuntime{}, errors.Wrap(err, "parse step settings")
	}

	runtimeKey := strings.TrimSpace(req.ProfileKey)
	if runtimeKey == "" {
		runtimeKey = strings.TrimSpace(c.options.RuntimeKey)
	}
	if runtimeKey == "" {
		runtimeKey = "inventory"
	}

	profileRuntime := req.ResolvedProfileRuntime
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

	mwFactories := map[string]infruntime.MiddlewareBuilder{
		hypercardPolicyMiddlewareName: func(cfg any) gepmw.Middleware {
			if c, ok := cfg.(InventoryArtifactPolicyConfig); ok {
				return NewInventoryArtifactPolicyMiddleware(c)
			}
			return NewInventoryArtifactPolicyMiddleware(InventoryArtifactPolicyConfig{})
		},
		hypercardSuggestionsMiddlewareName: func(cfg any) gepmw.Middleware {
			if c, ok := cfg.(InventorySuggestionsPolicyConfig); ok {
				return NewInventorySuggestionsPolicyMiddleware(c)
			}
			return NewInventorySuggestionsPolicyMiddleware(InventorySuggestionsPolicyConfig{})
		},
	}
	middlewares := []infruntime.MiddlewareSpec{
		{
			Name:   hypercardPolicyMiddlewareName,
			Config: InventoryArtifactPolicyConfig{},
		},
		{
			Name:   hypercardSuggestionsMiddlewareName,
			Config: InventorySuggestionsPolicyConfig{},
		},
	}

	engine_, err := infruntime.BuildEngineFromSettings(ctx, stepSettings.Clone(), systemPrompt, middlewares, mwFactories)
	if err != nil {
		return infruntime.ComposedRuntime{}, errors.Wrap(err, "compose engine")
	}

	return infruntime.ComposedRuntime{
		Engine:     engine_,
		RuntimeKey: runtimeKey,
		RuntimeFingerprint: runtimeFingerprint(runtimeFingerprintInput{
			ProfileVersion: req.ProfileVersion,
			RuntimeKey:     runtimeKey,
			SystemPrompt:   systemPrompt,
			AllowedTools:   allowedTools,
			ProfileRuntime: profileRuntime,
			StepSettings:   stepSettings,
		}),
		SeedSystemPrompt: systemPrompt,
		AllowedTools:     allowedTools,
	}, nil
}

type runtimeFingerprintInput struct {
	ProfileVersion uint64
	RuntimeKey     string
	SystemPrompt   string
	AllowedTools   []string
	ProfileRuntime *gepprofiles.RuntimeSpec
	StepSettings   *settings.StepSettings
}

func runtimeFingerprint(in runtimeFingerprintInput) string {
	payload := map[string]any{
		"profile_version": in.ProfileVersion,
		"runtime_key":     in.RuntimeKey,
		"system_prompt":   in.SystemPrompt,
		"allowed_tools":   in.AllowedTools,
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
