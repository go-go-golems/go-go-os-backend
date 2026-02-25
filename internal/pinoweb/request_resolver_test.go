package pinoweb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/stretchr/testify/require"
)

func TestStrictRequestResolver_WSRequiresConvID(t *testing.T) {
	r := NewStrictRequestResolver("inventory")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	_, err := r.Resolve(req)
	require.Error(t, err)

	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusBadRequest, re.Status)
}

func TestStrictRequestResolver_ChatRejectsOverrides(t *testing.T) {
	r := NewStrictRequestResolver("inventory")
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"prompt":"hi","overrides":{"system_prompt":"x"}}`))

	_, err := r.Resolve(req)
	require.Error(t, err)

	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusBadRequest, re.Status)
}

func TestStrictRequestResolver_ChatUsesTextFallback(t *testing.T) {
	r := NewStrictRequestResolver("inventory")
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"text":"hello"}`))

	plan, err := r.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "hello", plan.Prompt)
	require.NotEmpty(t, plan.ConvID)
	require.Equal(t, "inventory", plan.RuntimeKey)
}

func TestStrictRequestResolver_ChatUsesProfileSelection(t *testing.T) {
	r := newResolverWithProfiles(t)
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"text":"hello","profile":"analyst"}`))

	plan, err := r.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "hello", plan.Prompt)
	require.Equal(t, "analyst", plan.RuntimeKey)
	require.Equal(t, uint64(7), plan.ProfileVersion)
	require.NotNil(t, plan.ResolvedRuntime)
	require.Equal(t, "Analyst system", plan.ResolvedRuntime.SystemPrompt)
}

func TestStrictRequestResolver_WSUsesCookieProfileSelection(t *testing.T) {
	r := newResolverWithProfiles(t)
	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1", nil)
	req.AddCookie(&http.Cookie{Name: "chat_profile", Value: "analyst"})

	plan, err := r.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "analyst", plan.RuntimeKey)
	require.Equal(t, uint64(7), plan.ProfileVersion)
}

func TestStrictRequestResolver_UnknownProfileReturnsNotFound(t *testing.T) {
	r := newResolverWithProfiles(t)
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"prompt":"hi","profile":"missing"}`))

	_, err := r.Resolve(req)
	require.Error(t, err)
	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusNotFound, re.Status)
}

func TestStrictRequestResolver_UnknownRegistryReturnsNotFound(t *testing.T) {
	r := newResolverWithProfiles(t)
	req := httptest.NewRequest(http.MethodPost, "/chat?registry=missing", strings.NewReader(`{"prompt":"hi"}`))

	_, err := r.Resolve(req)
	require.Error(t, err)
	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusNotFound, re.Status)
}

func newResolverWithProfiles(t *testing.T) *StrictRequestResolver {
	t.Helper()

	store := gepprofiles.NewInMemoryProfileStore()
	registry := &gepprofiles.ProfileRegistry{
		Slug:               gepprofiles.MustRegistrySlug("default"),
		DefaultProfileSlug: gepprofiles.MustProfileSlug("inventory"),
		Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{
			gepprofiles.MustProfileSlug("inventory"): {
				Slug: gepprofiles.MustProfileSlug("inventory"),
				Runtime: gepprofiles.RuntimeSpec{
					SystemPrompt: "Inventory system",
				},
				Metadata: gepprofiles.ProfileMetadata{Version: 3},
			},
			gepprofiles.MustProfileSlug("analyst"): {
				Slug: gepprofiles.MustProfileSlug("analyst"),
				Runtime: gepprofiles.RuntimeSpec{
					SystemPrompt: "Analyst system",
				},
				Metadata: gepprofiles.ProfileMetadata{Version: 7},
			},
		},
	}
	require.NoError(t, gepprofiles.ValidateRegistry(registry))
	require.NoError(t, store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{
		Actor:  "test",
		Source: "test",
	}))

	svc, err := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug("default"))
	require.NoError(t, err)
	return NewStrictRequestResolver("inventory").WithProfileRegistry(svc, gepprofiles.MustRegistrySlug("default"))
}
