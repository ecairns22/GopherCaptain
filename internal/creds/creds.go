package creds

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Generate produces a random alphanumeric string of the given length using crypto/rand.
func Generate(length int) (string, error) {
	result := make([]byte, length)
	max := big.NewInt(int64(len(charset)))

	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generating random credential: %w", err)
		}
		result[i] = charset[n.Int64()]
	}

	return string(result), nil
}

// EnvFileContent holds key-value pairs for an environment file.
type EnvFileContent struct {
	Entries map[string]string
}

// WriteEnvFile writes KEY=VALUE lines to path with chmod 600.
func WriteEnvFile(path string, content *EnvFileContent) error {
	var lines []string
	// Sort keys for deterministic output
	keys := make([]string, 0, len(content.Entries))
	for k := range content.Entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, content.Entries[k]))
	}

	data := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		return fmt.Errorf("writing env file %s: %w", path, err)
	}
	return nil
}

// WriteTOMLConfigFile writes key-value pairs as TOML to path with chmod 600.
func WriteTOMLConfigFile(path string, content *EnvFileContent) error {
	var lines []string
	keys := make([]string, 0, len(content.Entries))
	for k := range content.Entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s = %q", k, content.Entries[k]))
	}

	data := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		return fmt.Errorf("writing TOML config file %s: %w", path, err)
	}
	return nil
}
