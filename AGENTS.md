# AGENTS.md

## 项目背景

docker-image-sync 是一个将公共容器镜像仓库（Docker Hub、registry.k8s.io、quay.io 等）的镜像自动同步到私有 Harbor/OCI 仓库的命令行工具。

核心使用场景：在内网或受限网络环境中，通过代理从公共仓库拉取镜像，推送到内部 Harbor 仓库供内网使用。支持增量同步（仅同步目标不存在的标签）、失败重试、多架构镜像复制、镜像清理（删除不匹配的镜像）。

## 技术栈

- **语言**: Go 1.25+
- **CLI 框架**: [cobra](https://github.com/spf13/cobra)
- **镜像操作**: [containers/image v5](https://github.com/containers/image) — skopeo 的底层库
  - `copy.Image()` — 镜像复制，无需安装 skopeo 二进制
  - `docker.GetRepositoryTags()` — 源标签列表获取，自动处理 OCI v2 Auth 认证流程
- **配置解析**: [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)
- **构建约束**: 必须使用 `-tags "containers_image_openpgp"` 编译，以使用纯 Go OpenPGP 实现替代 CGO 依赖的 GPGME

## 项目结构

```
├── main.go                              # 入口，调用 cmd.Execute()
├── cmd/
│   ├── root.go                          # 根命令、统一卡片和标签输出函数
│   ├── sync.go                          # sync 子命令 - 执行镜像同步，支持 --dry-run / --force
│   ├── list.go                          # list 子命令 - 列出目标仓库标签，支持 --online
│   └── delete.go                        # delete 子命令 - 删除不匹配镜像，支持 --dry-run / --online
├── internal/
│   ├── cfg/
│   │   ├── config.go                    # 配置结构体定义、校验、规则过滤
│   │   └── default.go                   # 配置文件加载路径查找
│   ├── hub/
│   │   ├── types.go                     # Client 接口
│   │   ├── client.go                    # containers/image 客户端
│   │   └── image.go                     # 镜像引用解析
│   ├── sync/
│   │   ├── types.go                     # TagSet, RuleSum 类型
│   │   └── syncer.go                    # 标签分析、镜像复制、删除执行
│   └── logger/
│       ├── logger.go                    # 彩色日志工具、标签分组显示、颜色常量
│       └── card.go                      # 卡片格式输出（PrintInfoCard）
├── config.yaml.example                  # 配置文件示例
├── Dockerfile                           # 多阶段 Docker 构建
└── go.mod / go.sum                      # Go 模块定义
```

## 详细文档

| 文档     | 路径                                               | 说明                                            |
| ------ | ------------------------------------------------ | --------------------------------------------- |
| 架构与数据流 | [docs/architecture.md](docs/architecture.md)     | 数据流概览、代理机制、认证机制                               |
| 配置模型   | [docs/configuration.md](docs/configuration.md)   | 全局配置、规则配置、镜像引用解析、校验与过滤                        |
| CLI 命令 | [docs/cli-commands.md](docs/cli-commands.md)     | sync/list/delete 子命令详细说明                |
| 关键实现细节 | [docs/implementation.md](docs/implementation.md) | Schema1 处理、同步流程、镜像复制、Harbor 客户端、标签过滤、logger 包 |

## 构建与部署

自动化验证使用 `./config-test.yaml` 配置文件
不允许使用本地 `./config.yaml` 配置文件。
编译时必须使用参数 `-tags "containers_image_openpgp"` 和 `CGO_ENABLED=0`

```bash
# 本地构建
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o bin/image-syncer .

# 测试运行
CGO_ENABLED=0 go run -tags "containers_image_openpgp" . sync -c ./config-test.yaml --dry-run

# 静态分析
CGO_ENABLED=0 go vet -tags "containers_image_openpgp" .

# 测试验证
./bin/image-syncer sync -c ./config-test.yaml --dry-run

# Docker 构建
docker build -t docker-image-sync:latest .

# 容器运行
docker run --rm \
  -v /path/to/config.yaml:/root/.config/docker-image-sync/config.yaml:ro \
  -v ~/.docker/config.json:/root/.docker/config.json:ro \
  docker-image-sync:latest sync
```

Docker 运行时需要挂载 `~/.docker/config.json` 以提供源仓库和 Harbor 的认证信息。
