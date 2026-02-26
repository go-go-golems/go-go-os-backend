package pinoweb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/google/uuid"
)

type StrictRequestResolver struct {
	runtimeKey          string
	profileRegistry     gepprofiles.Registry
	defaultRegistrySlug gepprofiles.RegistrySlug
}

type strictChatRequestBody struct {
	Prompt           string         `json:"prompt"`
	Text             string         `json:"text,omitempty"`
	ConvID           string         `json:"conv_id"`
	RuntimeKey       string         `json:"runtime_key,omitempty"`
	RegistrySlug     string         `json:"registry_slug,omitempty"`
	RequestOverrides map[string]any `json:"request_overrides"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
}

func NewStrictRequestResolver(runtimeKey string) *StrictRequestResolver {
	key := strings.TrimSpace(runtimeKey)
	if key == "" {
		key = "inventory"
	}
	return &StrictRequestResolver{
		runtimeKey:          key,
		defaultRegistrySlug: gepprofiles.MustRegistrySlug("default"),
	}
}

func (r *StrictRequestResolver) WithProfileRegistry(profileRegistry gepprofiles.Registry, registrySlug gepprofiles.RegistrySlug) *StrictRequestResolver {
	if r == nil {
		return nil
	}
	r.profileRegistry = profileRegistry
	if !registrySlug.IsZero() {
		r.defaultRegistrySlug = registrySlug
	}
	return r
}

func (r *StrictRequestResolver) Resolve(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	if req == nil {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "bad request",
		}
	}

	switch req.Method {
	case http.MethodGet:
		return r.resolveWS(req)
	case http.MethodPost:
		return r.resolveChat(req)
	default:
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
			Status:    http.StatusMethodNotAllowed,
			ClientMsg: "method not allowed",
		}
	}
}

func (r *StrictRequestResolver) resolveWS(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "missing conv_id",
		}
	}

	runtimeKey := r.runtimeKey
	var resolvedRuntime *gepprofiles.RuntimeSpec
	var profileVersion uint64
	if r.profileRegistry != nil {
		profileSlug, err := r.resolveProfileSelection(req, "")
		if err != nil {
			return webhttp.ResolvedConversationRequest{}, err
		}
		resolvedProfile, err := r.resolveEffectiveProfile(context.Background(), profileSlug, nil)
		if err != nil {
			return webhttp.ResolvedConversationRequest{}, err
		}
		runtime := resolvedProfile.EffectiveRuntime
		resolvedRuntime = &runtime
		runtimeKey = resolvedProfile.RuntimeKey.String()
		profileVersion = profileVersionFromResolvedMetadata(resolvedProfile.Metadata)
	}

	return webhttp.ResolvedConversationRequest{
		ConvID:          convID,
		RuntimeKey:      runtimeKey,
		ProfileVersion:  profileVersion,
		ResolvedRuntime: resolvedRuntime,
		Overrides:       nil,
	}, nil
}

func (r *StrictRequestResolver) resolveChat(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	var body strictChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "bad request",
			Err:       err,
		}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}

	runtimeKey := r.runtimeKey
	var resolvedRuntime *gepprofiles.RuntimeSpec
	var profileVersion uint64
	if r.profileRegistry != nil {
		profileSlug, err := r.resolveProfileSelection(req, body.RuntimeKey)
		if err != nil {
			return webhttp.ResolvedConversationRequest{}, err
		}
		resolvedProfile, err := r.resolveEffectiveProfile(context.Background(), profileSlug, body.RequestOverrides)
		if err != nil {
			return webhttp.ResolvedConversationRequest{}, err
		}
		runtime := resolvedProfile.EffectiveRuntime
		resolvedRuntime = &runtime
		runtimeKey = resolvedProfile.RuntimeKey.String()
		profileVersion = profileVersionFromResolvedMetadata(resolvedProfile.Metadata)
	}

	return webhttp.ResolvedConversationRequest{
		ConvID:          convID,
		RuntimeKey:      runtimeKey,
		ProfileVersion:  profileVersion,
		ResolvedRuntime: resolvedRuntime,
		Prompt:          body.Prompt,
		Overrides:       nil,
		IdempotencyKey:  strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func (r *StrictRequestResolver) resolveProfileSelection(
	req *http.Request,
	bodyRuntimeKeyRaw string,
) (gepprofiles.ProfileSlug, error) {
	if r == nil || r.profileRegistry == nil {
		return "", &webhttp.RequestResolutionError{
			Status:    http.StatusInternalServerError,
			ClientMsg: "profile resolver is not configured",
		}
	}

	slugRaw := strings.TrimSpace(bodyRuntimeKeyRaw)
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("runtime_key"))
	}
	if slugRaw == "" && req != nil {
		if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
			slugRaw = strings.TrimSpace(ck.Value)
		}
	}
	if strings.TrimSpace(slugRaw) == "" {
		return "", nil
	}

	slug, err := gepprofiles.ParseProfileSlug(slugRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "invalid runtime_key: " + slugRaw,
			Err:       err,
		}
	}
	return slug, nil
}

func (r *StrictRequestResolver) resolveEffectiveProfile(
	ctx context.Context,
	profileSlug gepprofiles.ProfileSlug,
	requestOverrides map[string]any,
) (*gepprofiles.ResolvedProfile, error) {
	in := gepprofiles.ResolveInput{
		ProfileSlug:      profileSlug,
		RequestOverrides: requestOverrides,
	}
	if !profileSlug.IsZero() {
		if runtimeKey, err := gepprofiles.ParseRuntimeKey(profileSlug.String()); err == nil {
			in.RuntimeKeyFallback = runtimeKey
		}
	}
	resolved, err := r.profileRegistry.ResolveEffectiveProfile(ctx, in)
	if err != nil {
		return nil, r.toRequestResolutionError(err, profileSlug.String())
	}
	return resolved, nil
}

func profileVersionFromResolvedMetadata(metadata map[string]any) uint64 {
	raw := metadata["profile.version"]
	switch v := raw.(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case uint:
		return uint64(v)
	case int64:
		if v >= 0 {
			return uint64(v)
		}
	case int:
		if v >= 0 {
			return uint64(v)
		}
	case float64:
		if v >= 0 {
			return uint64(v)
		}
	}
	return 0
}

func (r *StrictRequestResolver) toRequestResolutionError(err error, slug string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gepprofiles.ErrProfileNotFound) {
		msg := "profile not found"
		if strings.TrimSpace(slug) != "" {
			msg += ": " + slug
		}
		return &webhttp.RequestResolutionError{Status: http.StatusNotFound, ClientMsg: msg}
	}
	if errors.Is(err, gepprofiles.ErrRegistryNotFound) {
		return &webhttp.RequestResolutionError{
			Status:    http.StatusNotFound,
			ClientMsg: "registry not found",
			Err:       err,
		}
	}
	var validationErr *gepprofiles.ValidationError
	if errors.As(err, &validationErr) {
		return &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: validationErr.Error(),
			Err:       err,
		}
	}
	var policyErr *gepprofiles.PolicyViolationError
	if errors.As(err, &policyErr) {
		return &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: policyErr.Error(),
			Err:       err,
		}
	}
	return &webhttp.RequestResolutionError{
		Status:    http.StatusInternalServerError,
		ClientMsg: "profile resolution failed",
		Err:       err,
	}
}
