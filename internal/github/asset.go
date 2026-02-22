package github

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// ResolveAssetName executes the asset pattern template with the given parameters.
func ResolveAssetName(tmpl *template.Template, name, version string) (string, error) {
	var buf bytes.Buffer
	data := map[string]string{
		"Name":    name,
		"Version": version,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing asset pattern template: %w", err)
	}
	return buf.String(), nil
}

// FindAsset matches the expected name against available release assets.
// Returns the matching asset name or an error listing available names.
func FindAsset(assets []string, expected string) (string, error) {
	for _, a := range assets {
		if a == expected {
			return a, nil
		}
	}
	return "", fmt.Errorf("no asset matching %q found; available assets: %s", expected, strings.Join(assets, ", "))
}
