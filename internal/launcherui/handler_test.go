package launcherui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestSPAHandler_ServesExistingAsset(t *testing.T) {
	handler := newSPAHandler(fstest.MapFS{
		"index.html":     {Data: []byte("<html>launcher</html>")},
		"assets/main.js": {Data: []byte("console.log('launcher')")},
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/main.js", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "launcher")
}

func TestSPAHandler_FallsBackToIndexForClientRoutes(t *testing.T) {
	handler := newSPAHandler(fstest.MapFS{
		"index.html": {Data: []byte("<html><body>launcher-shell</body></html>")},
	})

	req := httptest.NewRequest(http.MethodGet, "/apps/inventory/window/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "launcher-shell")
}

func TestSPAHandler_DoesNotFallbackForMissingFileExtensions(t *testing.T) {
	handler := newSPAHandler(fstest.MapFS{
		"index.html": {Data: []byte("<html><body>launcher-shell</body></html>")},
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	body, err := io.ReadAll(rr.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(strings.ToLower(string(body)), "not found"))
}
