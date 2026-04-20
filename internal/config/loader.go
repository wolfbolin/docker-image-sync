package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	nameSet := make(map[string]bool)
	for i, rule := range c.Rules {
		if rule.Source == "" {
			return fmt.Errorf("rules[%d].source is required", i)
		}
		if rule.Dest == "" {
			return fmt.Errorf("rules[%d].destination is required", i)
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
		if len(rule.Tags) > 0 && rule.TagRegex != "" {
			return fmt.Errorf("rules[%d]: tags and tag_regex cannot be used together", i)
		}
	}

	return nil
}

func (c *Config) FilterRules(names string) ([]Rule, error) {
	if names == "" {
		return c.Rules, nil
	}

	nameList := strings.Split(names, ",")
	nameSet := make(map[string]bool, len(nameList))
	for _, n := range nameList {
		n = strings.TrimSpace(n)
		if n != "" {
			nameSet[n] = true
		}
	}

	var filtered []Rule
	for _, rule := range c.Rules {
		if nameSet[rule.Name] {
			filtered = append(filtered, rule)
			delete(nameSet, rule.Name)
		}
	}

	if len(nameSet) > 0 {
		var unknown []string
		for n := range nameSet {
			unknown = append(unknown, n)
		}
		return nil, fmt.Errorf("unknown rule names: %s", strings.Join(unknown, ", "))
	}

	return filtered, nil
}

func (c *Config) GetProxy() string {
	if c.Proxy != "" {
		return c.Proxy
	}
	if p := os.Getenv("HTTPS_PROXY"); p != "" {
		return p
	}
	if p := os.Getenv("HTTP_PROXY"); p != "" {
		return p
	}
	return ""
}

func (c *Config) GetNoProxy() string {
	if c.NoProxy != "" {
		return c.NoProxy
	}
	if v := os.Getenv("NO_PROXY"); v != "" {
		return v
	}
	if v := os.Getenv("no_proxy"); v != "" {
		return v
	}
	return ""
}

type ParsedRef struct {
	Registry string
	Project  string
	Name     string
}

func ParseRef(ref string) ParsedRef {
	parts := strings.SplitN(ref, "/", 3)
	switch len(parts) {
	case 1:
		return ParsedRef{Name: parts[0]}
	case 2:
		if isRegistry(parts[0]) {
			return ParsedRef{Registry: parts[0], Name: parts[1]}
		}
		return ParsedRef{Project: parts[0], Name: parts[1]}
	default:
		if isRegistry(parts[0]) {
			return ParsedRef{Registry: parts[0], Project: parts[1], Name: parts[2]}
		}
		return ParsedRef{Project: parts[0], Name: parts[1] + "/" + parts[2]}
	}
}

func isRegistry(part string) bool {
	return strings.Contains(part, ".") || strings.Contains(part, ":")
}

func BuildRef(pr ParsedRef, tag string) string {
	var parts []string
	if pr.Registry != "" {
		parts = append(parts, pr.Registry)
	}
	if pr.Project != "" {
		parts = append(parts, pr.Project)
	}
	parts = append(parts, pr.Name)
	return strings.Join(parts, "/") + ":" + tag
}
