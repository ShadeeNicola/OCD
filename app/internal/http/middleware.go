package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

// NewCORSMiddleware returns a handler middleware that applies the allowed origins
// policy to incoming requests. Allowed origins may contain hostnames, host:port,
// full origin strings (including scheme), or the wildcard "*".
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	normalized, allowAll := normalizeAllowedOrigins(allowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Always provide standard headers so preflight responses are predictable.
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Add("Vary", "Origin")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")

			if origin == "" {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if originAllowed(origin, normalized) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
				} else {
					http.Error(w, "CORS origin denied", http.StatusForbidden)
				}
				return
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func normalizeAllowedOrigins(origins []string) ([]string, bool) {
	var normalized []string
	allowAll := false

	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			allowAll = true
			continue
		}
		normalized = append(normalized, strings.ToLower(trimmed))
	}

	return normalized, allowAll
}

func originAllowed(origin string, allowed []string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	lowerOrigin := strings.ToLower(origin)
	hostWithPort := strings.ToLower(parsed.Host)
	host := strings.ToLower(parsed.Hostname())

	for _, allowedOrigin := range allowed {
		if allowedOrigin == "" {
			continue
		}

		if lowerOrigin == allowedOrigin {
			return true
		}

		if hostWithPort == allowedOrigin {
			return true
		}

		if host == allowedOrigin {
			return true
		}

		if strings.HasPrefix(allowedOrigin, "http://") || strings.HasPrefix(allowedOrigin, "https://") {
			if lowerOrigin == allowedOrigin {
				return true
			}
		}
	}

	return false
}
