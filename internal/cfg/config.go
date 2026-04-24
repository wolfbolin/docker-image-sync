package cfg

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/wolfbolin/bolbox/pkg/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Proxy string      `yaml:"proxy"`
	Retry RetryConfig `yaml:"retry"`
	Rules []Rule      `yaml:"rules"`
}

type RetryConfig struct {
	Times    int           `yaml:"times"`
	Interval time.Duration `yaml:"interval"`
}

type Rule struct {
	Name     string   `yaml:"name"`
	Source   string   `yaml:"source"`
	Target   string   `yaml:"target"`
	Proxy    bool     `yaml:"proxy"`
	Tags     []string `yaml:"tags"`
	TagRegex string   `yaml:"tag_regex"`
}

func NewConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err = cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.Proxy == "" {
		cfg.LoadProxyByEnv()
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	// 校验基础配置
	if c.Retry.Times <= 0 {
		log.Warn("retry.times is reset to 3")
		c.Retry.Times = 3
	}
	if c.Retry.Interval <= 0 {
		log.Warn("retry.interval is reset to 5 second")
		c.Retry.Interval = 5 * time.Second
	}

	if c.Proxy != "" {
		if _, err := url.Parse(c.Proxy); err != nil {
			return fmt.Errorf("proxy is invalid url: %w", err)
		}
	}

	// 校验 Rule
	nameSet := make(map[string]bool)
	for i, rule := range c.Rules {
		if rule.Source == "" {
			return fmt.Errorf("rules[%d].source is required", i)
		}
		if rule.Target == "" {
			return fmt.Errorf("rules[%d].target is required", i)
		}
		if rule.Name != "" {
			if nameSet[rule.Name] {
				return fmt.Errorf("rules[%d]: duplicate rule name %q", i, rule.Name)
			}
			nameSet[rule.Name] = true
		}
		if rule.TagRegex != "" {
			if _, err := regexp.Compile(rule.TagRegex); err != nil {
				return fmt.Errorf("rules[%d].tag_regex is invalid regex: %w", i, err)
			}
		}
	}

	return nil
}

func (c *Config) LoadProxyByEnv() string {
	if p := os.Getenv("HTTPS_PROXY"); p != "" {
		return p
	}
	if p := os.Getenv("HTTP_PROXY"); p != "" {
		return p
	}
	return ""
}

func (c *Config) FilterRules(names string) {
	if names == "" {
		return
	}

	specNames := strings.Split(names, ",")
	filteredRules := make([]Rule, 0)
	for _, rule := range c.Rules {
		if slices.Contains(specNames, rule.Name) {
			filteredRules = append(filteredRules, rule)
		}
	}
	c.Rules = filteredRules
}
