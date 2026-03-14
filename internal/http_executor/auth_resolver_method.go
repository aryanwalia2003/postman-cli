package http_executor

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"reqx/internal/collection"
)

// ApplyAuth injects authentication headers/params into the HTTP request.
// It is a pure function — it does not modify the auth config itself.
func ApplyAuth(req *http.Request, auth *collection.Auth) {
	if auth == nil || strings.ToLower(auth.Type) == "none" {
		return
	}
	switch strings.ToLower(auth.Type) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	case "basic":
		encoded := base64.StdEncoding.EncodeToString([]byte(auth.Username + ":" + auth.Password))
		req.Header.Set("Authorization", "Basic "+encoded)
	case "apikey":
		applyAPIKey(req, auth)
	case "cookie":
		applyCookies(req, auth.Cookies)
	}
}

func applyAPIKey(req *http.Request, auth *collection.Auth) {
	if strings.ToLower(auth.In) == "query" {
		q := req.URL.Query()
		q.Set(auth.Key, auth.Value)
		req.URL.RawQuery = q.Encode()
		return
	}
	req.Header.Set(auth.Key, auth.Value)
}

func applyCookies(req *http.Request, cookies map[string]string) {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	if len(parts) > 0 {
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
}
