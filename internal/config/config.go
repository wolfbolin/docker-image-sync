package config

import "time"

type Config struct {
	Proxy   string     `yaml:"proxy"`
	NoProxy string     `yaml:"no_proxy"`
	Sync    SyncConfig `yaml:"sync"`
	Rules   []Rule     `yaml:"rules"`
}

type SyncConfig struct {
	Retry    int           `yaml:"retry"`
	Interval time.Duration `yaml:"interval"`
}

type Rule struct {
	Name     string   `yaml:"name"`
	Source   string   `yaml:"source"`
	Dest     string   `yaml:"destination"`
	Proxy    bool     `yaml:"proxy"`
	Tags     []string `yaml:"tags"`
	TagRegex string   `yaml:"tag_regex"`
}

func (s *SyncConfig) RetryDuration() time.Duration {
	if s.Interval <= 0 {
		return 5 * time.Second
	}
	return s.Interval
}

func (s *SyncConfig) RetryCount() int {
	if s.Retry <= 0 {
		return 3
	}
	return s.Retry
}
