package ratelimit

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Enabled        bool         `yaml:"enabled"`
	ExemptAccounts []string     `yaml:"exemptAccounts"`
	RedisTimeout   int          `yaml:"redisTimeout"` // backend timeout in milliseconds before fail-open (default 20)
	Default        RouteLimit   `yaml:"default"`
	Routes         []RouteLimit `yaml:"routes"`
	exemptSet      map[string]struct{}
}

type RouteLimit struct {
	Path   string `yaml:"path"`   // URL path pattern (e.g. "/api/v0/clusters/*")
	Method string `yaml:"method"` // HTTP method (GET, POST, etc.)
	Rate   int    `yaml:"rate"`   // allowed requests per window
	Burst  int    `yaml:"burst"`  // max burst above rate (defaults to rate * 2)
	Window int    `yaml:"window"` // window duration in seconds (default 1)
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading rate limit config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing rate limit config: %w", err)
	}

	if cfg.Default.Rate < 0 {
		return nil, fmt.Errorf("default rate must be >= 0, got %d", cfg.Default.Rate)
	}
	if cfg.Default.Rate == 0 {
		cfg.Default.Rate = 100
	}
	if cfg.Default.Burst < 0 {
		return nil, fmt.Errorf("default burst must be >= 0, got %d", cfg.Default.Burst)
	}
	if cfg.Default.Burst == 0 {
		cfg.Default.Burst = cfg.Default.Rate * 2
	} else if cfg.Default.Burst < cfg.Default.Rate {
		slog.Warn("default burst < rate may cause unexpected denials",
			"burst", cfg.Default.Burst, "rate", cfg.Default.Rate)
	}
	if cfg.Default.Window < 0 {
		return nil, fmt.Errorf("default window must be >= 0, got %d", cfg.Default.Window)
	}
	if cfg.Default.Window == 0 {
		cfg.Default.Window = 1
	}
	if cfg.RedisTimeout <= 0 {
		cfg.RedisTimeout = 20
	}

	for i, r := range cfg.Routes {
		if r.Rate <= 0 {
			return nil, fmt.Errorf("route %d (%s %s): rate must be > 0", i, r.Method, r.Path)
		}
		if r.Burst < 0 {
			return nil, fmt.Errorf("route %d (%s %s): burst must be >= 0", i, r.Method, r.Path)
		}
		if r.Burst == 0 {
			cfg.Routes[i].Burst = r.Rate * 2
		} else if r.Burst < r.Rate {
			slog.Warn("route burst < rate may cause unexpected denials",
				"route", r.Path, "method", r.Method, "burst", r.Burst, "rate", r.Rate)
		}
		if r.Window < 0 {
			return nil, fmt.Errorf("route %d (%s %s): window must be >= 0", i, r.Method, r.Path)
		}
		if r.Window == 0 {
			cfg.Routes[i].Window = cfg.Default.Window
		}
	}

	cfg.exemptSet = make(map[string]struct{}, len(cfg.ExemptAccounts))
	for _, acc := range cfg.ExemptAccounts {
		cfg.exemptSet[acc] = struct{}{}
	}

	return &cfg, nil
}

func (c *Config) isExempt(accountID string) bool {
	_, ok := c.exemptSet[accountID]
	return ok
}

func NewDefaultConfig(rate, burst, window int) *Config {
	if rate <= 0 {
		rate = 100
	}
	if burst <= 0 {
		burst = rate * 2
	}
	if window <= 0 {
		window = 1
	}
	return &Config{
		Enabled:      true,
		RedisTimeout: 20,
		Default:      RouteLimit{Rate: rate, Burst: burst, Window: window},
		exemptSet:    map[string]struct{}{},
	}
}
