package config

import (
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const defaultConfigPath = "/etc/gophercaptain/gophercaptain.conf"
const envOverride = "GOPHERCAPTAIN_CONFIG"

type Config struct {
	GitHub   GitHubConfig   `toml:"github"`
	Ports    PortsConfig    `toml:"ports"`
	MariaDB  MariaDBConfig  `toml:"mariadb"`
	Nginx    NginxConfig    `toml:"nginx"`
	Releases ReleasesConfig `toml:"releases"`
}

type GitHubConfig struct {
	Token string `toml:"token"`
	Owner string `toml:"owner"`
}

type PortsConfig struct {
	RangeStart int `toml:"range_start"`
	RangeEnd   int `toml:"range_end"`
}

type MariaDBConfig struct {
	Host              string `toml:"host"`
	Port              int    `toml:"port"`
	AdminUser         string `toml:"admin_user"`
	AdminPasswordFile string `toml:"admin_password_file"`
	AdminPassword     string `toml:"-"` // resolved at load time, never serialized
}

type NginxConfig struct {
	SitesDir   string `toml:"sites_dir"`
	EnabledDir string `toml:"enabled_dir"`
}

type ReleasesConfig struct {
	AssetPattern string `toml:"asset_pattern"`
}

// DefaultPath returns the default configuration file path.
func DefaultPath() string {
	if p := os.Getenv(envOverride); p != "" {
		return p
	}
	return defaultConfigPath
}

// Load reads configuration from the default path.
func Load() (*Config, error) {
	return LoadFrom(DefaultPath())
}

// LoadFrom reads configuration from the given path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Apply defaults
	if cfg.Ports.RangeStart == 0 {
		cfg.Ports.RangeStart = 3000
	}
	if cfg.Ports.RangeEnd == 0 {
		cfg.Ports.RangeEnd = 4000
	}
	if cfg.MariaDB.Host == "" {
		cfg.MariaDB.Host = "127.0.0.1"
	}
	if cfg.MariaDB.Port == 0 {
		cfg.MariaDB.Port = 3306
	}
	if cfg.MariaDB.AdminUser == "" {
		cfg.MariaDB.AdminUser = "root"
	}
	if cfg.Nginx.SitesDir == "" {
		cfg.Nginx.SitesDir = "/etc/nginx/sites-available"
	}
	if cfg.Nginx.EnabledDir == "" {
		cfg.Nginx.EnabledDir = "/etc/nginx/sites-enabled"
	}
	if cfg.Releases.AssetPattern == "" {
		cfg.Releases.AssetPattern = "{{.Name}}-linux-amd64"
	}

	// Validate required fields
	if cfg.GitHub.Token == "" {
		return nil, fmt.Errorf("config: github.token is required")
	}
	if cfg.GitHub.Owner == "" {
		return nil, fmt.Errorf("config: github.owner is required")
	}

	// Resolve MariaDB admin password from file
	if cfg.MariaDB.AdminPasswordFile == "" {
		return nil, fmt.Errorf("config: mariadb.admin_password_file is required")
	}
	pwData, err := os.ReadFile(cfg.MariaDB.AdminPasswordFile)
	if err != nil {
		return nil, fmt.Errorf("reading mariadb admin password from %s: %w", cfg.MariaDB.AdminPasswordFile, err)
	}
	cfg.MariaDB.AdminPassword = strings.TrimSpace(string(pwData))
	if cfg.MariaDB.AdminPassword == "" {
		return nil, fmt.Errorf("config: mariadb admin password file %s is empty", cfg.MariaDB.AdminPasswordFile)
	}

	return &cfg, nil
}

// TemplateConfig returns a TOML template with placeholder values for first-time setup.
func TemplateConfig() string {
	return `[github]
token = "ghp_YOUR_TOKEN_HERE"
owner = "your-github-username"

[ports]
range_start = 3000
range_end   = 4000

[mariadb]
host     = "127.0.0.1"
port     = 3306
admin_user = "root"
admin_password_file = "/root/.mariadb_password"

[nginx]
sites_dir   = "/etc/nginx/sites-available"
enabled_dir = "/etc/nginx/sites-enabled"

[releases]
asset_pattern = "{{.Name}}-linux-amd64"
`
}
