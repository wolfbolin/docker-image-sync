# SyncDockerHub

将 Docker Hub 及其他公共仓库的容器镜像自动同步到私有 Harbor 仓库的命令行工具。

## 功能特性

### 独立规则配置

- 每条规则独立配置源镜像和目标仓库，支持同步到不同的 Harbor 实例和项目
- 支持从 Docker Hub（`docker.io`）、`registry.k8s.io`、`quay.io` 等 OCI 兼容仓库同步
- 通过 YAML 配置文件定义同步规则，无需数据库

### 灵活的标签匹配

- **精确标签列表**（`tags`）：指定多个标签进行同步
- **正则匹配**（`tag_regex`）：使用正则表达式批量匹配标签

### 按规则代理控制

- 每条规则可独立声明是否使用代理（`proxy: true/false`）
- 全局代理支持配置文件和环境变量（`HTTPS_PROXY`/`HTTP_PROXY`）兜底

### 智能同步策略

- 自动跳过目标仓库中已存在的标签，避免重复同步
- 自动跳过已废弃的 V1 manifest 格式镜像
- 单个标签同步失败不影响整体流程，继续处理后续标签
- 同步失败自动重试，可配置重试次数和间隔

### 规则筛选

- `sync`、`check`、`list` 命令均支持 `--rule` / `-r` 参数指定规则名称
- 支持逗号分隔指定多条规则，不指定则执行全部

### 预览检查

- `check` 命令可预览规则匹配结果，展示匹配标签、已存在标签和待同步标签，不执行实际同步操作

### 内置镜像复制

- 基于 [containers/image](https://github.com/containers/image) 库实现镜像复制，无需安装 skopeo 运行时依赖
- 支持多架构镜像同步（`--all`）和摘要保留（`--preserve-digests`）

## 依赖

- Go 1.25+

## 构建

```bash
# 本地构建
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o sync-docker .

# 交叉编译 Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "containers_image_openpgp" -o sync-docker .

# Docker 多阶段构建
docker build -t sync-docker:latest .
```

## 配置

复制示例配置并按需修改：

```bash
cp config.yaml.example config.yaml
```

配置文件结构：

```yaml
# 全局代理（可选，支持环境变量 HTTPS_PROXY/HTTP_PROXY 兜底）
proxy: "http://1.2.3.4:5678"

# 同步行为
sync:
  retry: 3
  interval: 5s

# 同步规则 - 每条规则独立配置源和目标
rules:
  # 精确标签列表同步
  - name: "alpine"
    source: "docker.io/library/alpine"
    destination: "mirror.com/docker.io/alpine"
    proxy: true
    tags: ["3.19", "3.20"]

  # 正则匹配同步
  - name: "nginx"
    source: "docker.io/library/nginx"
    destination: "mirror.com/docker.io/nginx"
    proxy: false
    tag_regex: "^1\\.2[0-9]\\..*$"

  # 不同目标仓库同步
  - name: "kube-apiserver"
    source: "registry.k8s.io/kube-apiserver"
    destination: "mirror.com/k8s.io/kube-apiserver"
    proxy: true
    tag_regex: "^v1\\.(28|29|30)\\.[0-9]+$"
```

### 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 否 | 规则名称，用于 `--rule` 参数筛选，必须唯一 |
| `source` | 是 | 源镜像路径，格式为 `registry/project/name` |
| `destination` | 是 | 目标镜像路径，格式为 `registry/project/name` |
| `proxy` | 是 | 是否使用全局代理拉取镜像 |
| `tags` | 否* | 精确标签列表，与 `tag_regex` 二选一 |
| `tag_regex` | 否* | 正则表达式匹配标签，与 `tags` 二选一 |

> *`tags` 和 `tag_regex` 不可同时使用，至少配置一种。

### source / destination 格式

使用 `/` 分隔，支持以下格式：

| 格式 | 示例 | 说明 |
|------|------|------|
| `name` | `nginx` | 仅镜像名 |
| `project/name` | `library/nginx` | 项目 + 镜像名 |
| `registry/project/name` | `docker.io/library/nginx` | 完整路径（推荐） |

## 使用

```bash
# 执行同步（所有规则）
sync-docker sync -c config.yaml

# 执行同步（指定规则）
sync-docker sync -c config.yaml -r alpine,nginx

# 预览规则匹配结果（不实际同步）
sync-docker check -c config.yaml

# 预览指定规则
sync-docker check -c config.yaml -r alpine

# 查看已同步的镜像和标签
sync-docker list -c config.yaml

# 查看指定规则的已同步镜像
sync-docker list -c config.yaml -r alpine
```

### sync 命令输出示例

```
[INFO] Config loaded successfully
[INFO] Start sync, 2 rules in total
[INFO] [Rule 1/2] alpine => mirror..com/docker.io/alpine
[INFO]   Sync: docker.io/library/alpine:3.19 => mirror..com/docker.io/alpine:3.19
[DONE]   ✓ Success
[INFO] [Rule 2/2] nginx => mirror..com/docker.io/nginx
[WARN]   Skip: 1.20.0 (exists)
[INFO]   Sync: docker.io/library/nginx:1.21.0 => mirror..com/docker.io/nginx:1.21.0
[DONE]   ✓ Success
[INFO] Sync complete: Success=2 Failed=0 Exist=1 Skip=0
```

### check 命令输出示例

```
╭ Rule 1/2 ──────────────────────────────────────────────╮
│ Name:        alpine                                │
│ Source:      docker.io/library/alpine              │
│ Destination: mirror..com/docker.io/alpine          │
│ Mode:        tags (exact match)                    │
╰──────────────────────────────────────────────────────────╯
  ✓ Will sync (2):  3.19  3.20
  ● Already exist (0):  -
  ○ Skipped V1 (0):  -

╭ Rule 2/2 ──────────────────────────────────────────────╮
│ Name:        nginx                                       │
│ Source:      docker.io/library/nginx                     │
│ Destination: mirror..com/docker.io/nginx                 │
│ Mode:        tag_regex                                   │
│ Pattern:     ^1\.2[0-9]\..*$                             │
│ Total tags:  156                                         │
╰──────────────────────────────────────────────────────────╯
  ✓ Will sync (3):  1.20.1  1.20.2  1.21.0
  ● Already exist (1):  1.20.0
  ○ Skipped V1 (0):  -
```

### list 命令输出示例

```
List images for 2 rule(s):
========================================

  alpine (docker.io/library/alpine => mirror..com/docker.io/alpine)
    - 3.19
    - 3.20

  nginx (docker.io/library/nginx => mirror..com/docker.io/nginx)
    - 1.20.0
    - 1.20.1
    - 1.21.0

========================================
```

## Docker 部署

```bash
docker run --rm \
  -v /path/to/config.yaml:/etc/sync-docker/config.yaml:ro \
  -v ~/.docker/config.json:/root/.docker/config.json:ro \
  sync-docker:latest
```

需要挂载 `~/.docker/config.json` 以提供 Harbor 写入认证。可通过 Kubernetes CronJob 或 cron 定时执行。

## 项目结构

```
├── main.go                         # 入口
├── cmd/                            # CLI 命令（sync, check, list）
├── internal/
│   ├── config/                     # 配置加载、校验、规则过滤
│   ├── registry/                   # Docker Hub / Harbor API 客户端
│   ├── syncer/                     # 同步引擎、标签过滤
│   └── logger/                     # 日志输出
├── config.yaml.example             # 配置示例
└── Dockerfile                      # 多阶段构建
```
