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

		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				// Director never called this, so client-supplied
				// X-Forwarded-* headers were passed through unsanitized;
				// Rewrite's replacement strips them before setting fresh
				// values, closing that spoofing path.
				pr.SetXForwarded()
				path := strings.TrimPrefix(r.URL.Path, coreUIProxyPrefix)
				if path == "" {
					path = "/"
				}
				pr.Out.URL.Path = path
				pr.Out.URL.RawPath = ""
				pr.Out.Host = target.Host
				pr.Out.SetBasicAuth(credentials.Username, credentials.Password)
				pr.Out.Header.Set("X-Forwarded-Prefix", publicUIProxyPrefix)
			},
			ModifyResponse: rewriteAdGuardUIResponse,
			ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, proxyErr error) {
				http.Error(writer, fmt.Sprintf("AdGuard UI gateway: %v", proxyErr), http.StatusBadGateway)
			},
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
		for _, raw := range cookies {
			response.Header.Add("Set-Cookie", rewriteAdGuardSetCookie(raw))
		}
	}
	return nil
}

// rewriteAdGuardSetCookie repoints a root-scoped cookie from AdGuard's own
// Path=/ to this proxy's mount point, so the browser keeps sending it back
// on later requests that all live under publicUIProxyPrefix instead of at
// the real root. Found in review: the previous implementation did a plain
// strings.Replace looking for the literal substring "Path=/;", which
// missed a cookie where Path=/ is the *last* attribute (no trailing
// semicolon - a completely ordinary, spec-legal Set-Cookie shape), any
// other attribute-name casing ("path=/"), or extra whitespace. Parsing the
// cookie properly (net/http already implements RFC 6265 for this) and
// only touching the Path field sidesteps all of that, and leaves every
// other attribute (Secure, HttpOnly, SameSite, Expires, ...) intact
// either way. A cookie whose Path isn't exactly "/" - or that fails to
// parse at all - is passed through unchanged, matching what the old code
// did for those cases too.
func rewriteAdGuardSetCookie(raw string) string {
	cookie, err := http.ParseSetCookie(raw)
	if err != nil || cookie.Path != "/" {
		return raw
	}
	cookie.Path = publicUIProxyPrefix + "/"
	rewritten := cookie.String()
	// Cookie.Unparsed holds any attribute-value pair ParseSetCookie
	// couldn't map to one of its own known fields (a vendor-specific or
	// not-yet-standard attribute, verified live: an unrecognized
	// "FutureAttr=xyz" lands here) - found in a follow-up review:
	// Cookie.String() only ever serializes the fields it recognizes, so
	// this rewrite was silently dropping anything in Unparsed instead of
	// round-tripping it. Order doesn't matter for Set-Cookie attributes
	// (RFC 6265), so appending them back on is sufficient.
	for _, pair := range cookie.Unparsed {
		rewritten += "; " + pair
	}
	return rewritten
}
