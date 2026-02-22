package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/template"

	gh "github.com/google/go-github/v60/github"
)

const binBase = "/opt/gophercaptain/bin"

// Client wraps the GitHub API for release operations.
type Client struct {
	gh           *gh.Client
	httpClient   *http.Client
	defaultOwner string
	assetTmpl    *template.Template
}

// New creates a GitHub client with the given token, default owner, and asset pattern.
func New(token, defaultOwner, assetPattern string) (*Client, error) {
	tmpl, err := template.New("asset").Parse(assetPattern)
	if err != nil {
		return nil, fmt.Errorf("parsing asset pattern %q: %w", assetPattern, err)
	}

	httpClient := &http.Client{}
	ghClient := gh.NewClient(httpClient).WithAuthToken(token)

	return &Client{
		gh:           ghClient,
		httpClient:   httpClient,
		defaultOwner: defaultOwner,
		assetTmpl:    tmpl,
	}, nil
}

// newWithClients creates a Client with injected HTTP and GitHub clients (for testing).
func newWithClients(ghClient *gh.Client, httpClient *http.Client, defaultOwner, assetPattern string) (*Client, error) {
	tmpl, err := template.New("asset").Parse(assetPattern)
	if err != nil {
		return nil, fmt.Errorf("parsing asset pattern %q: %w", assetPattern, err)
	}
	return &Client{
		gh:           ghClient,
		httpClient:   httpClient,
		defaultOwner: defaultOwner,
		assetTmpl:    tmpl,
	}, nil
}

// ResolveVersion resolves "latest" to the actual release tag, or returns the version as-is.
func (c *Client) ResolveVersion(ctx context.Context, owner, repo, version string) (string, error) {
	if owner == "" {
		owner = c.defaultOwner
	}

	if version == "latest" || version == "" {
		release, _, err := c.gh.Repositories.GetLatestRelease(ctx, owner, repo)
		if err != nil {
			return "", fmt.Errorf("getting latest release for %s/%s: %w", owner, repo, err)
		}
		return release.GetTagName(), nil
	}
	return version, nil
}

// DownloadAsset downloads the matching release asset to /opt/gophercaptain/bin/<name>/<name>-<version>,
// makes it executable, and creates a symlink <name> -> <name>-<version>.
func (c *Client) DownloadAsset(ctx context.Context, owner, repo, version, serviceName string) (string, error) {
	if owner == "" {
		owner = c.defaultOwner
	}

	// Get the release
	release, _, err := c.gh.Repositories.GetReleaseByTag(ctx, owner, repo, version)
	if err != nil {
		return "", fmt.Errorf("getting release %s for %s/%s: %w", version, owner, repo, err)
	}

	// Resolve expected asset name
	expected, err := ResolveAssetName(c.assetTmpl, serviceName, version)
	if err != nil {
		return "", err
	}

	// Collect asset names and find matching one
	var assetNames []string
	var matchedAsset *gh.ReleaseAsset
	for _, a := range release.Assets {
		name := a.GetName()
		assetNames = append(assetNames, name)
		if name == expected {
			matchedAsset = a
		}
	}

	if matchedAsset == nil {
		_, findErr := FindAsset(assetNames, expected)
		return "", findErr
	}

	// Download
	rc, _, err := c.gh.Repositories.DownloadReleaseAsset(ctx, owner, repo, matchedAsset.GetID(), c.httpClient)
	if err != nil {
		return "", fmt.Errorf("downloading asset %s: %w", expected, err)
	}
	defer rc.Close()

	// Write to disk
	dir := filepath.Join(binBase, serviceName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating bin dir %s: %w", dir, err)
	}

	filename := fmt.Sprintf("%s-%s", serviceName, version)
	destPath := filepath.Join(dir, filename)

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("creating file %s: %w", destPath, err)
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		return "", fmt.Errorf("writing asset to %s: %w", destPath, err)
	}
	f.Close()

	// chmod +x
	if err := os.Chmod(destPath, 0755); err != nil {
		return "", fmt.Errorf("chmod %s: %w", destPath, err)
	}

	// Create/update symlink
	symlinkPath := filepath.Join(dir, serviceName)
	os.Remove(symlinkPath) // remove existing symlink if any
	if err := os.Symlink(filename, symlinkPath); err != nil {
		return "", fmt.Errorf("creating symlink %s -> %s: %w", symlinkPath, filename, err)
	}

	return destPath, nil
}
