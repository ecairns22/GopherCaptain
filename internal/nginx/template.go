package nginx

import (
	"bytes"
	"fmt"
	"text/template"
)

const subdomainTemplate = `server {
    listen 80;
    server_name {{.RouteValue}};

    location / {
        proxy_pass http://127.0.0.1:{{.Port}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`

const pathTemplate = `location {{.RouteValue}} {
    proxy_pass http://127.0.0.1:{{.Port}};
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
`

var parsedSubdomainTemplate = template.Must(template.New("subdomain").Parse(subdomainTemplate))
var parsedPathTemplate = template.Must(template.New("path").Parse(pathTemplate))

// RouteParams holds values for rendering nginx config.
type RouteParams struct {
	Name       string
	RouteType  string // "subdomain" or "path"
	RouteValue string
	Port       int
}

// RenderConfig renders the nginx config for the given route parameters.
func RenderConfig(params RouteParams) (string, error) {
	var buf bytes.Buffer
	var tmpl *template.Template

	switch params.RouteType {
	case "subdomain":
		tmpl = parsedSubdomainTemplate
	case "path":
		tmpl = parsedPathTemplate
	default:
		return "", fmt.Errorf("unknown route type %q; must be 'subdomain' or 'path'", params.RouteType)
	}

	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("rendering nginx config: %w", err)
	}
	return buf.String(), nil
}
