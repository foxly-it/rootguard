package adguard

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const coreUIProxyPrefix = "/api/adguard/ui"
const publicUIProxyPrefix = "/adguard-ui"

func (m *Manager) UIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credentials, err := m.loadCredentials()
		if err != nil {
			http.Error(w, fmt.Sprintf("load AdGuard UI credentials: %v", err), http.StatusServiceUnavailable)
			return
		}
		target, err := url.Parse(m.apiURL)
		if err != nil {
			http.Error(w, "invalid AdGuard UI upstream", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(request *http.Request) {
			originalDirector(request)
			path := strings.TrimPrefix(r.URL.Path, coreUIProxyPrefix)
			if path == "" {
				path = "/"
			}
			request.URL.Path = path
			request.URL.RawPath = ""
			request.Host = target.Host
			request.SetBasicAuth(credentials.Username, credentials.Password)
			request.Header.Set("X-Forwarded-Prefix", publicUIProxyPrefix)
		}
		proxy.ModifyResponse = rewriteAdGuardUIResponse
		proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, proxyErr error) {
			http.Error(writer, fmt.Sprintf("AdGuard UI gateway: %v", proxyErr), http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	})
}

func rewriteAdGuardUIResponse(response *http.Response) error {
	if location := response.Header.Get("Location"); location != "" {
		if parsed, err := url.Parse(location); err == nil {
			if parsed.IsAbs() || strings.HasPrefix(parsed.Path, "/") {
				parsed.Scheme = ""
				parsed.Host = ""
				parsed.Path = publicUIProxyPrefix + "/" + strings.TrimPrefix(parsed.Path, "/")
				response.Header.Set("Location", parsed.String())
			}
		}
	}
	cookies := response.Header.Values("Set-Cookie")
	if len(cookies) > 0 {
		response.Header.Del("Set-Cookie")
		for _, cookie := range cookies {
			cookie = strings.Replace(cookie, "Path=/;", "Path="+publicUIProxyPrefix+"/;", 1)
			response.Header.Add("Set-Cookie", cookie)
		}
	}
	return nil
}
