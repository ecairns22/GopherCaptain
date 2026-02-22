package systemd

import (
	"bytes"
	"text/template"
)

const unitTemplate = `[Unit]
Description=GopherCaptain: {{.Name}}
After=network.target mariadb.service

[Service]
Type=simple
ExecStart=/opt/gophercaptain/bin/{{.Name}}/{{.Name}}
EnvironmentFile=/etc/gophercaptain/{{.Name}}/env
Restart=on-failure
RestartSec=5
User=gc-{{.Name}}
Group=gc-{{.Name}}
WorkingDirectory=/opt/gophercaptain/bin/{{.Name}}

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/gophercaptain/bin/{{.Name}}

[Install]
WantedBy=multi-user.target
`

var parsedUnitTemplate = template.Must(template.New("unit").Parse(unitTemplate))

// ServiceParams holds values for the systemd unit template.
type ServiceParams struct {
	Name string
}

// RenderUnit renders the systemd unit file for the given parameters.
func RenderUnit(params ServiceParams) (string, error) {
	var buf bytes.Buffer
	if err := parsedUnitTemplate.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}
