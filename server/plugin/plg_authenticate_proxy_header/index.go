// Proxy-header auth: read the authenticated identity from an HTTP header
// (e.g. Remote-User), optionally map it to a local (OMV/Linux) username, and
// pass it to the backend.
//
// Exposed to Admin → Backend → Attribute mapping:
//
//	{{ .user }}     the mapped local username (use this for Samba/SFTP login)
//	{{ .identity }} the raw value from the proxy header (e.g. an email address)
//
// Set the backend password in the attribute mapping (Password field).
package plg_authenticate_proxy_header

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/mickael-kerjean/filestash/server/common"
)

const tokenTTL = 2 * time.Minute

// Per-process key used to sign the hand-off between EntryPoint and Callback,
// so a client cannot POST an arbitrary username to /api/session/auth/.
var signingKey = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("plg_authenticate_proxy_header: cannot generate signing key")
	}
	return b
}()

func init() {
	Hooks.Register.AuthenticationMiddleware("proxy_header", ProxyHeader{})
}

type ProxyHeader struct{}

func (this ProxyHeader) Setup() Form {
	return Form{
		Elmnts: []FormElement{
			{Name: "type", Type: "hidden", Value: "proxy_header"},
			{
				Name:        "header_name",
				Type:        "text",
				Value:       "Remote-User",
				Description: "HTTP header that contains the authenticated username.",
			},
			{
				Name: "user_map",
				Type: "long_text",
				Placeholder: `alice@example.com = alice
bob@example.com = bob`,
				Default:     "",
				Description: `Optional. Map the identity from the proxy header to a local username, one per line as 'identity = localuser'. Matching is case-insensitive. Lines starting with # are ignored. Use {{ .user }} (mapped name) or {{ .identity }} (raw header value) in the attribute mapping.`,
			},
			{
				Name:  "unmapped",
				Type:  "select",
				Value: "passthrough",
				Opts:  []string{"passthrough", "strip_domain", "deny"},
				Description: `What to do when the identity is not in the user map:
- passthrough: use the header value as-is (original behaviour)
- strip_domain: use the part before '@' (alice.smith@example.com → alice.smith)
- deny: refuse the login (the user map becomes an allowlist)`,
			},
		},
	}
}

func parseUserMap(raw string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(parts[0]))
		v := strings.TrimSpace(parts[1])
		if k != "" && v != "" {
			m[k] = v
		}
	}
	return m
}

// resolveUser returns the local username for an identity, or "" if denied.
func resolveUser(identity string, idpParams map[string]string) string {
	if local, ok := parseUserMap(idpParams["user_map"])[strings.ToLower(identity)]; ok {
		return local
	}
	switch idpParams["unmapped"] {
	case "deny":
		return ""
	case "strip_domain":
		if i := strings.Index(identity, "@"); i > 0 {
			return identity[:i]
		}
		return identity
	default:
		return identity
	}
}

func sign(payload string) string {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// token = base64(identity) "." base64(user) "." expiry "." signature
func makeToken(identity, user string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(identity)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(user)) + "." +
		strconv.FormatInt(time.Now().Add(tokenTTL).Unix(), 10)
	return payload + "." + sign(payload)
}

func readToken(token string) (identity, user string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return "", "", false
	}
	payload := strings.Join(parts[:3], ".")
	if !hmac.Equal([]byte(sign(payload)), []byte(parts[3])) {
		return "", "", false
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", "", false
	}
	i, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	u, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	if err1 != nil || err2 != nil {
		return "", "", false
	}
	return string(i), string(u), true
}

func deny(res http.ResponseWriter, msg string) {
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	res.WriteHeader(http.StatusUnauthorized)
	res.Write([]byte(Page(`<h1>Access denied</h1><p>` + html.EscapeString(msg) + `</p>`)))
}

func (this ProxyHeader) EntryPoint(idpParams map[string]string, req *http.Request, res http.ResponseWriter) error {
	headerName := idpParams["header_name"]
	if headerName == "" {
		headerName = "Remote-User"
	}
	identity := strings.TrimSpace(req.Header.Get(headerName))
	if identity == "" {
		deny(res, "No proxy user header present. Log in via the reverse proxy first.")
		return nil
	}
	user := resolveUser(identity, idpParams)
	if user == "" {
		Log.Warning("plg_authenticate_proxy_header: no mapping for identity '%s'", identity)
		deny(res, "Your account is not allowed to use this server.")
		return nil
	}
	q := "?label=" + html.EscapeString(req.URL.Query().Get("label")) + "&state=" + html.EscapeString(req.URL.Query().Get("state"))
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(Page(`
		<form id="f" action="` + WithBase("/api/session/auth/"+q) + `" method="post">
			<input type="hidden" name="token" value="` + html.EscapeString(makeToken(identity, user)) + `" />
		</form>
		<script>document.getElementById("f").submit();</script>
	`)))
	return nil
}

func (this ProxyHeader) Callback(formData map[string]string, idpParams map[string]string, res http.ResponseWriter) (map[string]string, error) {
	identity, user, ok := readToken(strings.TrimSpace(formData["token"]))
	if !ok || user == "" {
		return nil, ErrNotAllowed
	}
	return map[string]string{
		"user":     user,
		"identity": identity,
	}, nil
}
