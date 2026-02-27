package backendhost

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var validAppIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

var forbiddenLegacyAliases = map[string]struct{}{
	"/chat":         {},
	"/ws":           {},
	"/api/timeline": {},
}

func ValidateAppID(appID string) error {
	id := strings.TrimSpace(appID)
	if id == "" {
		return fmt.Errorf("app id is required")
	}
	if !validAppIDPattern.MatchString(id) {
		return fmt.Errorf("app id must match %s", validAppIDPattern.String())
	}
	return nil
}

func NamespacedAppPrefix(appID string) (string, error) {
	id := strings.TrimSpace(appID)
	if err := ValidateAppID(id); err != nil {
		return "", err
	}
	return "/api/apps/" + id, nil
}

func MountNamespacedRoutes(parent *http.ServeMux, appID string, mount func(mux *http.ServeMux) error) error {
	if parent == nil {
		return fmt.Errorf("parent mux is nil")
	}
	if mount == nil {
		return fmt.Errorf("mount callback is nil")
	}
	prefix, err := NamespacedAppPrefix(appID)
	if err != nil {
		return err
	}

	subMux := http.NewServeMux()
	if err := mount(subMux); err != nil {
		return err
	}

	parent.Handle(prefix, http.StripPrefix(prefix, subMux))
	parent.Handle(prefix+"/", http.StripPrefix(prefix, subMux))
	return nil
}

func GuardNoLegacyAliases(paths []string) error {
	for _, raw := range paths {
		candidate := normalizeRoutePath(raw)
		if _, ok := forbiddenLegacyAliases[candidate]; ok {
			return fmt.Errorf("legacy route alias %q is forbidden; use /api/apps/<app-id> namespace", raw)
		}
	}
	return nil
}

func normalizeRoutePath(path string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return ""
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	for strings.Contains(normalized, "//") {
		normalized = strings.ReplaceAll(normalized, "//", "/")
	}
	if len(normalized) > 1 {
		normalized = strings.TrimRight(normalized, "/")
	}
	return normalized
}
