package pinoweb

import (
	"context"
	"encoding/json"
	"strings"

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

func (c *RuntimeComposer) Compose(ctx context.Context, req infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
	if c == nil || c.parsed == nil {
		return infruntime.RuntimeArtifacts{}, errors.New("runtime composer is not configured")
	}
	if ctx == nil {
		return infruntime.RuntimeArtifacts{}, errors.New("compose context is nil")
	}

	if len(req.Overrides) > 0 {
		return infruntime.RuntimeArtifacts{}, errors.New("runtime overrides are not allowed for inventory")
	}

	stepSettings, err := settings.NewStepSettingsFromParsedValues(c.parsed)
	if err != nil {
		return infruntime.RuntimeArtifacts{}, errors.Wrap(err, "parse step settings")
	}

	systemPrompt := strings.TrimSpace(c.options.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = "You are an inventory assistant."
	}

	engine_, err := infruntime.ComposeEngineFromSettings(ctx, stepSettings.Clone(), systemPrompt, nil, map[string]infruntime.MiddlewareFactory{})
	if err != nil {
		return infruntime.RuntimeArtifacts{}, errors.Wrap(err, "compose engine")
	}

	runtimeKey := strings.TrimSpace(c.options.RuntimeKey)
	if runtimeKey == "" {
		runtimeKey = "inventory"
	}

	allowedTools := append([]string(nil), c.options.AllowedTools...)
	return infruntime.RuntimeArtifacts{
		Engine:             engine_,
		RuntimeKey:         runtimeKey,
		RuntimeFingerprint: runtimeFingerprint(runtimeKey, systemPrompt, allowedTools, stepSettings),
		SeedSystemPrompt:   systemPrompt,
		AllowedTools:       allowedTools,
	}, nil
}

func runtimeFingerprint(runtimeKey, systemPrompt string, allowedTools []string, stepSettings *settings.StepSettings) string {
	payload := map[string]any{
		"runtime_key":   runtimeKey,
		"system_prompt": systemPrompt,
		"allowed_tools": allowedTools,
	}
	if stepSettings != nil {
		payload["step_metadata"] = stepSettings.GetMetadata()
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return runtimeKey
	}
	return string(b)
}
