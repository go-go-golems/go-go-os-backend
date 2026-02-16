package pinoweb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
