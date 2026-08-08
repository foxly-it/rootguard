package routerimport

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// digestChallenge holds the parameters a router sends back in a
// WWW-Authenticate: Digest header (RFC 7616), enough to answer the
// qop=auth, MD5 variant TR-064 routers use.
type digestChallenge struct {
	realm  string
	nonce  string
	opaque string
	qop    string
}

func parseDigestChallenge(header string) (digestChallenge, bool) {
	const prefix = "Digest "
	if !strings.HasPrefix(header, prefix) {
		return digestChallenge{}, false
	}
	params := parseAuthParams(header[len(prefix):])
	challenge := digestChallenge{
		realm:  params["realm"],
		nonce:  params["nonce"],
		opaque: params["opaque"],
		qop:    preferredQop(params["qop"]),
	}
	if challenge.realm == "" || challenge.nonce == "" {
		return digestChallenge{}, false
	}
	return challenge, true
}

// preferredQop picks "auth" out of a comma-separated qop-options list if
// offered; TR-064 routers only ever offer "auth", never "auth-int".
func preferredQop(value string) string {
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) == "auth" {
			return "auth"
		}
	}
	return ""
}

// parseAuthParams splits comma-separated key=value pairs while respecting
// quoted values that may themselves contain commas.
func parseAuthParams(value string) map[string]string {
	params := make(map[string]string)
	var current strings.Builder
	inQuotes := false
	flush := func() {
		key, val, found := strings.Cut(current.String(), "=")
		if found {
			params[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(val), `"`)
		}
		current.Reset()
	}
	for _, r := range value {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case r == ',' && !inQuotes:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return params
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomHex(byteLength int) (string, error) {
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// authorizationHeader builds the RFC 7616 Authorization header answering
// this challenge for the given credentials, HTTP method, and request URI.
func (c digestChallenge) authorizationHeader(username, password, method, uri string) (string, error) {
	cnonce, err := randomHex(8)
	if err != nil {
		return "", fmt.Errorf("generate client nonce: %w", err)
	}
	ha1 := md5Hex(username + ":" + c.realm + ":" + password)
	ha2 := md5Hex(method + ":" + uri)

	var response, qopFields string
	if c.qop == "auth" {
		const nc = "00000001"
		response = md5Hex(strings.Join([]string{ha1, c.nonce, nc, cnonce, c.qop, ha2}, ":"))
		qopFields = fmt.Sprintf(`, qop=%s, nc=%s, cnonce=%q`, c.qop, nc, cnonce)
	} else {
		response = md5Hex(strings.Join([]string{ha1, c.nonce, ha2}, ":"))
	}

	header := fmt.Sprintf(`Digest username=%q, realm=%q, nonce=%q, uri=%q, response=%q`,
		username, c.realm, c.nonce, uri, response) + qopFields
	if c.opaque != "" {
		header += fmt.Sprintf(`, opaque=%q`, c.opaque)
	}
	return header, nil
}
