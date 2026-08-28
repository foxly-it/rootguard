package httpapi

import (
	"net/http"
	"strings"
)

// themeScriptCSPHash is the CSP script-src hash for the tiny inline
// flash-of-wrong-theme-prevention script in frontend/index.html. It must
// run before the stylesheet paints, so it can't be an external file
// without reintroducing the flash it exists to avoid - a hash lets CSP
// allow exactly this one inline script instead of falling back to
// 'unsafe-inline' for the whole page. Vite passes this <head> script
// through untouched (verified against frontend/dist/index.html), so the
// hash stays valid across builds; if that script's text ever changes,
// recompute this with:
//
//	sha256sum of the exact bytes between <script> and </script>, base64-encoded
const themeScriptCSPHash = "sha256-+C0x7/zuFuw+CpbiMSrto5eJiTTTdncf3epdtyCvJS0="

// contentSecurityPolicy locks script execution down to same-origin files
// plus the one hashed inline script above - the actual XSS-relevant
// restriction. style-src keeps 'unsafe-inline': React applies the style
// prop a handful of components use (AdGuard.tsx and others) via the
// CSSStyleDeclaration API rather than the HTML style attribute, which CSP
// style-src doesn't reliably distinguish from attribute-based inline
// styles across browsers, and getting that wrong would break rendering
// silently in the field - not worth the risk for a directive that isn't
// the one guarding against script injection.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' '" + themeScriptCSPHash + "'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// SecurityHeaders adds the browser-side hardening headers a security
// review found missing entirely: without them, a same-origin script
// injection bug elsewhere (XSS) has no defense-in-depth backstop, and the
// admin UI can be framed by another site (clickjacking) and its referrer
// leaked to whatever it links out to. None of this replaces
// RequireSameOriginWrites or SessionAuth - it's the layer that limits the
// blast radius if something upstream of those ever fails.
//
// The Content-Security-Policy is withheld for /adguard-ui/ - that path is
// AdGuard Home's own admin UI, reverse-proxied through unchanged (see
// coreclient.Client.AdGuardUIHandler). It's a separate, third-party
// frontend we don't control the scripts/styles of, so our SPA's own
// script-src hash and style-src policy would be as likely to break it as
// to protect it. The other headers below don't depend on knowing that
// app's asset layout, so they still apply there too.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		if !strings.HasPrefix(r.URL.Path, "/adguard-ui/") {
			header.Set("Content-Security-Policy", contentSecurityPolicy)
		}
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "same-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}
