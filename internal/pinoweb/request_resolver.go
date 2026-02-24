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
		_, slug, profile, err := r.resolveProfileSelection(req, "", "")
		if err != nil {
			return webhttp.ResolvedConversationRequest{}, err
		}
		runtimeKey = slug.String()
		resolvedRuntime = profileRuntimeSpec(profile)
		if profile != nil {
			profileVersion = profile.Metadata.Version
		}
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
	var body webhttp.ChatRequestBody
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
	if len(body.Overrides) > 0 {
		return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "runtime overrides are not allowed",
		}
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}

	runtimeKey := r.runtimeKey
	var resolvedRuntime *gepprofiles.RuntimeSpec
	var profileVersion uint64
	if r.profileRegistry != nil {
		_, slug, profile, err := r.resolveProfileSelection(req, body.Profile, body.Registry)
		if err != nil {
			return webhttp.ResolvedConversationRequest{}, err
		}
		runtimeKey = slug.String()
		resolvedRuntime = profileRuntimeSpec(profile)
		if profile != nil {
			profileVersion = profile.Metadata.Version
		}
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
	bodyProfileRaw string,
	bodyRegistryRaw string,
) (gepprofiles.RegistrySlug, gepprofiles.ProfileSlug, *gepprofiles.Profile, error) {
	if r == nil || r.profileRegistry == nil {
		return "", "", nil, &webhttp.RequestResolutionError{
			Status:    http.StatusInternalServerError,
			ClientMsg: "profile resolver is not configured",
		}
	}

	registrySlug, err := r.resolveRegistrySlug(req, bodyRegistryRaw)
	if err != nil {
		return "", "", nil, err
	}

	slugRaw := strings.TrimSpace(bodyProfileRaw)
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("profile"))
	}
	if slugRaw == "" && req != nil {
		slugRaw = strings.TrimSpace(req.URL.Query().Get("runtime"))
	}
	if slugRaw == "" && req != nil {
		if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
			slugRaw = strings.TrimSpace(ck.Value)
		}
	}

	ctx := context.Background()
	if slugRaw == "" {
		defaultSlug, err := r.resolveDefaultProfileSlug(ctx, registrySlug)
		if err != nil {
			return "", "", nil, r.toRequestResolutionError(err, "")
		}
		slugRaw = defaultSlug.String()
	}

	slug, err := gepprofiles.ParseProfileSlug(slugRaw)
	if err != nil {
		return "", "", nil, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "invalid profile: " + slugRaw,
			Err:       err,
		}
	}

	profile, err := r.profileRegistry.GetProfile(ctx, registrySlug, slug)
	if err != nil {
		return "", "", nil, r.toRequestResolutionError(err, slugRaw)
	}
	return registrySlug, slug, profile, nil
}

func (r *StrictRequestResolver) resolveRegistrySlug(req *http.Request, bodyRegistryRaw string) (gepprofiles.RegistrySlug, error) {
	registryRaw := strings.TrimSpace(bodyRegistryRaw)
	if registryRaw == "" && req != nil {
		registryRaw = strings.TrimSpace(req.URL.Query().Get("registry"))
	}
	if registryRaw == "" {
		if r == nil || r.defaultRegistrySlug.IsZero() {
			return gepprofiles.MustRegistrySlug("default"), nil
		}
		return r.defaultRegistrySlug, nil
	}

	registrySlug, err := gepprofiles.ParseRegistrySlug(registryRaw)
	if err != nil {
		return "", &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "invalid registry: " + registryRaw,
			Err:       err,
		}
	}
	return registrySlug, nil
}

func (r *StrictRequestResolver) resolveDefaultProfileSlug(ctx context.Context, registrySlug gepprofiles.RegistrySlug) (gepprofiles.ProfileSlug, error) {
	registry, err := r.profileRegistry.GetRegistry(ctx, registrySlug)
	if err != nil {
		return "", err
	}
	if registry != nil && !registry.DefaultProfileSlug.IsZero() {
		return registry.DefaultProfileSlug, nil
	}
	return gepprofiles.MustProfileSlug("default"), nil
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
	return &webhttp.RequestResolutionError{
		Status:    http.StatusInternalServerError,
		ClientMsg: "profile resolution failed",
		Err:       err,
	}
}

func profileRuntimeSpec(p *gepprofiles.Profile) *gepprofiles.RuntimeSpec {
	if p == nil {
		return nil
	}
	spec := gepprofiles.RuntimeSpec{
		StepSettingsPatch: map[string]any{},
		SystemPrompt:      strings.TrimSpace(p.Runtime.SystemPrompt),
		Middlewares:       append([]gepprofiles.MiddlewareUse(nil), p.Runtime.Middlewares...),
		Tools:             append([]string(nil), p.Runtime.Tools...),
	}
	for k, v := range p.Runtime.StepSettingsPatch {
		spec.StepSettingsPatch[k] = v
	}
	for i := range spec.Middlewares {
		mw := spec.Middlewares[i]
		config := mw.Config
		if config == nil {
			if fromExt, ok, err := gepprofiles.MiddlewareConfigFromExtensions(p.Extensions, mw, i); err == nil && ok {
				config = fromExt
			}
		}
		spec.Middlewares[i].Config = config
	}
	if len(spec.StepSettingsPatch) == 0 {
		spec.StepSettingsPatch = nil
	}
	return &spec
}
