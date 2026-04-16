package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "sync-docker",
	Short: "Docker image sync tool",
	Long:  "SyncDockerHub - Sync Docker Hub images to private Harbor registry",
}

func Execute() {
	findDefaultConfig()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
}

func findDefaultConfig() {
	// 如果已通过命令行参数指定配置，则不使用默认查找
	if cfgFile != "" {
		return
	}

	// 优先检查环境变量
	if env := os.Getenv("SYNC_DOCKER_CONFIG"); env != "" {
		cfgFile = env
		return
	}

	// 默认路径列表
	defaultPaths := []string{
		"/opt/docker-image-sync/config.yaml",
	}

	// 添加用户配置目录
	if home := os.Getenv("HOME"); home != "" {
		defaultPaths = append(defaultPaths, filepath.Join(home, ".config", "docker-image-sync", "config.yaml"))
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			cfgFile = path
			return
		}
	}
}
