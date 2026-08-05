package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

func RequireSameOriginWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		referer := r.Header.Get("Referer")
		fetchSite := r.Header.Get("Sec-Fetch-Site")
		if (origin == "" && referer == "" && fetchSite == "") ||
			originMatchesHost(origin, r.Host) ||
			originMatchesHost(referer, r.Host) ||
			strings.EqualFold(fetchSite, "same-origin") {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Cross-origin administration request rejected", http.StatusForbidden)
	})
}

func originMatchesHost(raw, host string) bool {
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	return err == nil && strings.EqualFold(parsed.Host, host)
}
