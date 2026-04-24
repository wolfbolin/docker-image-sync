package cfg

import (
	"os"
	"path/filepath"

	"github.com/wolfbolin/bolbox/pkg/log"
)

func LoadConfig(configFile string) *Config {
	configPaths := loadConfigPaths(configFile)

	for _, path := range configPaths {
		cfg, err := NewConfig(path)
		if err != nil {
			log.Warnf("Can not load config file from %s: %+v", path, err)
			continue
		}
		log.Infof("Load config file from %s", path)
		return cfg
	}
	log.Fatal("Can not load config file from all path.")
	os.Exit(1)
	return nil
}

func loadConfigPaths(configFile string) []string {
	var configPaths []string
	// 用户指定路径
	if len(configFile) != 0 {
		specPath, _ := filepath.Abs(configFile)
		configPaths = append(configPaths, specPath)
	}

	// 检查环境变量
	if env := os.Getenv("SYNC_DOCKER_CONFIG"); env != "" {
		configPaths = append(configPaths, env)
	}

	// 添加用户配置目录
	if home := os.Getenv("HOME"); home != "" {
		configPaths = append(configPaths, filepath.Join(home, ".config", "docker-image-sync", "config.yaml"))
	}

	// 本地默认路径
	if pwd, err := os.Getwd(); err == nil {
		configPaths = append(configPaths, filepath.Join(pwd, "config.yaml"))
	}

	return configPaths
}
