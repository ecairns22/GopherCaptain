package github

import (
	"strings"
	"testing"
	"text/template"
)

func TestResolveAssetNameDefault(t *testing.T) {
	tmpl, err := template.New("asset").Parse("{{.Name}}-linux-amd64")
	if err != nil {
		t.Fatal(err)
	}

	name, err := ResolveAssetName(tmpl, "myapi", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "myapi-linux-amd64" {
		t.Errorf("name = %q, want %q", name, "myapi-linux-amd64")
	}
}

func TestResolveAssetNameCustom(t *testing.T) {
	tmpl, err := template.New("asset").Parse("{{.Name}}_{{.Version}}_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	name, err := ResolveAssetName(tmpl, "myapi", "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "myapi_v2.0.0_linux_amd64.tar.gz" {
		t.Errorf("name = %q, want %q", name, "myapi_v2.0.0_linux_amd64.tar.gz")
	}
}

func TestFindAssetMatch(t *testing.T) {
	assets := []string{"myapi-linux-amd64", "myapi-darwin-arm64", "checksums.txt"}

	got, err := FindAsset(assets, "myapi-linux-amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "myapi-linux-amd64" {
		t.Errorf("got = %q, want %q", got, "myapi-linux-amd64")
	}
}

func TestFindAssetMiss(t *testing.T) {
	assets := []string{"myapi-darwin-arm64", "checksums.txt"}

	_, err := FindAsset(assets, "myapi-linux-amd64")
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
	if !strings.Contains(err.Error(), "myapi-darwin-arm64") {
		t.Errorf("error should list available assets, got: %v", err)
	}
	if !strings.Contains(err.Error(), "checksums.txt") {
		t.Errorf("error should list all available assets, got: %v", err)
	}
}
