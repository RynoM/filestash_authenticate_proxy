// Proxy-header auth: read username from HTTP header (e.g. Remote-User), pass to backend.
// Set password in Admin → Backend → Attribute mapping (Password field).
package plg_authenticate_proxy_header

import (
	"html"
	"net/http"
	"strings"

	. "github.com/mickael-kerjean/filestash/server/common"
)

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
		},
	}
}

func (this ProxyHeader) EntryPoint(idpParams map[string]string, req *http.Request, res http.ResponseWriter) error {
	headerName := idpParams["header_name"]
	if headerName == "" {
		headerName = "Remote-User"
	}
	user := strings.TrimSpace(req.Header.Get(headerName))
	if user == "" {
		res.Header().Set("Content-Type", "text/html; charset=utf-8")
		res.WriteHeader(http.StatusUnauthorized)
		res.Write([]byte(Page(`
			<h1>Access denied</h1>
			<p>No proxy user header present. Log in via the reverse proxy first.</p>
		`)))
		return nil
	}
	q := "?label=" + html.EscapeString(req.URL.Query().Get("label")) + "&state=" + html.EscapeString(req.URL.Query().Get("state"))
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(Page(`
		<form id="f" action="` + WithBase("/api/session/auth/"+q) + `" method="post">
			<input type="hidden" name="user" value="` + html.EscapeString(user) + `" />
		</form>
		<script>document.getElementById("f").submit();</script>
	`)))
	return nil
}

func (this ProxyHeader) Callback(formData map[string]string, idpParams map[string]string, res http.ResponseWriter) (map[string]string, error) {
	user := strings.TrimSpace(formData["user"])
	return map[string]string{"user": user}, nil
}
