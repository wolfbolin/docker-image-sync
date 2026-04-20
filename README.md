# SyncDockerHub

将公共容器镜像仓库的镜像自动同步到私有 Harbor 仓库的命令行工具。

## 支持的源仓库

| 源仓库 | registry 标识 | 认证方式 |
|--------|---------------|----------|
| Docker Hub | `docker.io` | OCI v2 Auth（自动匿名/Bearer/Basic） |
| registry.k8s.io | `registry.k8s.io` | OCI v2 Auth |
| Quay.io | `quay.io` | OCI v2 Auth |
| GitHub Container Registry | `ghcr.io` | OCI v2 Auth |
| 阿里云 ACR | `registry.aliyuncs.com` | OCI v2 Auth |
| 其他 OCI 兼容仓库 | 任意域名 | OCI v2 Auth |

> 所有源仓库统一使用 `containers/image` 库获取标签列表，自动处理认证流程（Bearer token 获取/刷新、Basic auth、匿名回退），认证信息从 `~/.docker/config.json` 读取。

## 功能特性

### 统一源仓库支持

- 基于 [containers/image](https://github.com/containers/image) 库统一获取所有源仓库的标签列表
- 自动处理 OCI v2 Auth 认证流程，无需为不同仓库适配认证逻辑
- 认证信息从 `~/.docker/config.json` 自动读取，无需额外配置

### 独立规则配置

- 每条规则独立配置源镜像和目标仓库，支持同步到不同的 Harbor 实例和项目
- 通过 YAML 配置文件定义同步规则，无需数据库

### 灵活的标签匹配

- **精确标签列表**（`tags`）：指定多个标签进行同步
- **正则匹配**（`tag_regex`）：使用正则表达式批量匹配标签

### 按规则代理控制

- 每条规则可独立声明是否使用代理（`proxy: true/false`）
- 全局代理支持配置文件和环境变量（`HTTPS_PROXY`/`HTTP_PROXY`）兜底
- 代理通过 `containers/image` 库的 `SystemContext.DockerProxyURL` 显式传递，仅作用于源端拉取，目标端推送直连
- 不依赖环境变量切换代理（Go 标准库 `http.ProxyFromEnvironment` 使用 `sync.Once` 缓存，运行时修改环境变量不生效）

### 智能同步策略

- 自动跳过目标仓库中已存在的标签，避免重复同步
- digest 变化时自动触发更新同步
- schema1 格式镜像自动跳过（不兼容 `PreserveDigests`）
- 单个标签同步失败不影响整体流程，继续处理后续标签
- 同步失败自动重试，可配置重试次数和间隔

### 镜像清理

- `delete` 命令删除目标仓库中不匹配规则的镜像和 schema1 格式镜像
- 支持 `tags` 模式（保留列表中的标签）和 `tag_regex` 模式（保留匹配正则的标签）
- `--dry-run` 参数预览删除计划，不实际执行删除
- Box 格式展示规则信息和删除计划

### 规则筛选

- 所有命令均支持 `--rule` / `-r` 参数指定规则名称
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
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o image-syncer .

# 交叉编译 Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "containers_image_openpgp" -o image-syncer .

# Docker 多阶段构建
docker build -t image-syncer:latest .
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

# 代理排除列表（保留字段，当前代理仅作用于源端，目标端直连无需排除）
no_proxy: "harbor.example.com,127.0.0.1,localhost"

# 同步行为
sync:
  retry: 3
  interval: 5s

# 同步规则 - 每条规则独立配置源和目标
rules:
  # Docker Hub - 精确标签列表同步
  - name: "alpine"
    source: "docker.io/library/alpine"
    destination: "harbor.example.com/docker.io/alpine"
    proxy: true
    tags: ["3.19", "3.20"]

  # Docker Hub - 正则匹配同步
  - name: "nginx"
    source: "docker.io/library/nginx"
    destination: "harbor.example.com/docker.io/nginx"
    proxy: false
    tag_regex: "^1\\.2[0-9]\\..*$"
```

### 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 否 | 规则名称，用于 `--rule` 参数筛选，必须唯一 |
| `source` | 是 | 源镜像路径，格式见下方说明 |
| `destination` | 是 | 目标镜像路径，格式为 `registry/project/name` |
| `proxy` | 是 | 是否使用全局代理拉取镜像 |
| `tags` | 否* | 精确标签列表，与 `tag_regex` 二选一 |
| `tag_regex` | 否* | 正则表达式匹配标签，与 `tags` 二选一 |

> *`tags` 和 `tag_regex` 不可同时使用，至少配置一种。

### source 格式说明

`source` 字段使用 `/` 分隔，程序自动识别 registry 域名（包含 `.` 或 `:` 的第一段）：

| 格式 | 示例 | 解析结果 |
|------|------|----------|
| 仅镜像名 | `nginx` | Name=nginx |
| project/name | `library/nginx` | Project=library, Name=nginx |
| registry/project/name | `docker.io/library/nginx` | Registry=docker.io, Project=library, Name=nginx |
| registry/name | `registry.k8s.io/pause` | Registry=registry.k8s.io, Name=pause |
| registry/project/name | `registry.k8s.io/coredns/coredns` | Registry=registry.k8s.io, Project=coredns, Name=coredns |
| registry:port/project/name | `harbor.example.com:23333/docker.io/ubuntu` | Registry=harbor.example.com:23333, Project=docker.io, Name=ubuntu |

> **推荐**：始终使用完整路径（含 registry 前缀），避免歧义。

## 使用

```bash
# 执行同步（所有规则）
image-syncer sync -c config.yaml

# 执行同步（指定规则）
image-syncer sync -c config.yaml -r alpine,nginx

# 预览规则匹配结果（不实际同步）
image-syncer check -c config.yaml

# 预览指定规则
image-syncer check -c config.yaml -r alpine

# 查看已同步的镜像和标签
image-syncer list -c config.yaml

# 查看指定规则的已同步镜像
image-syncer list -c config.yaml -r alpine

# 预览删除计划（dry run，不实际删除）
image-syncer delete -c config.yaml --dry-run

# 预览指定规则的删除计划
image-syncer delete -c config.yaml -r busybox --dry-run

# 执行删除
image-syncer delete -c config.yaml -r busybox
```

### sync 命令输出示例

```
[INFO] Config loaded successfully
[INFO] Start sync, 2 rules in total
[INFO] [Rule 1/2] alpine => harbor.example.com/docker.io/alpine
[INFO]   Sync: docker.io/library/alpine:3.19 => harbor.example.com/docker.io/alpine:3.19
[DONE]   ✓ Success
[INFO] [Rule 2/2] coredns => harbor.example.com/registry.k8s.io/coredns/coredns
[WARN]   Skip: v1.9.3 (up-to-date)
[INFO]   Sync: registry.k8s.io/coredns/coredns:v1.10.0 => harbor.example.com/registry.k8s.io/coredns/coredns:v1.10.0
[DONE]   ✓ Success
[INFO] Sync complete: Success=2 Failed=0 Exist=1 Skip=0
```

### check 命令输出示例

```
╭ Rule 1/2 ──────────────────────────────────────────────╮
│ Name:        alpine                                    │
│ Source:      docker.io/library/alpine                  │
│ Destination: harbor.example.com/docker.io/alpine       │
│ Mode:        tags (exact match)                        │
╰──────────────────────────────────────────────────────────╯
  ✓ Will sync (2):  3.19  3.20
  ● Already exist (0):  -
  ↻ Need update (0):  -

╭ Rule 2/2 ──────────────────────────────────────────────╮
│ Name:        coredns                                   │
│ Source:      registry.k8s.io/coredns/coredns           │
│ Destination: harbor.example.com/registry.k8s.io/coredns/coredns │
│ Mode:        tag_regex                                 │
│ Pattern:     ^v1\.(28|29|30)\.[0-9]+$                  │
│ Total tags:  42                                        │
╰──────────────────────────────────────────────────────────╯
  ✓ Will sync (3):  v1.28.5  v1.29.3  v1.30.0
  ● Already exist (1):  v1.9.3
  ↻ Need update (0):  -
```

### delete 命令输出示例

```
╭ Rule 1/1 ──────────────────────────────────────────────╮
│ Name:        ubuntu                                    │
│ Destination: harbor.example.com/docker.io/ubuntu       │
│ Mode:        tags (keep listed only)                   │
│ Keep tags:   22.04, 24.04                             │
│ Total tags:  21                                        │
│ Dry run:     true (no changes)                         │
╰──────────────────────────────────────────────────────────╯
  ✗ Schema1 (will delete) (0):  -
  ✗ Unmatched (will delete) (19): 24.10  23.10  23.04  22.10  ...
  ✓ Kept (2): 24.04  22.04
```

### list 命令输出示例

```
List images for 2 rule(s):
========================================

  alpine (docker.io/library/alpine => harbor.example.com/docker.io/alpine)
    - 3.19
    - 3.20

  coredns (registry.k8s.io/coredns/coredns => harbor.example.com/registry.k8s.io/coredns/coredns)
    - v1.9.3
    - v1.28.5
    - v1.29.3
    - v1.30.0

========================================
```

## Docker 部署

```bash
docker run --rm \
  -v /path/to/config.yaml:/etc/image-syncer/config.yaml:ro \
  -v ~/.docker/config.json:/root/.docker/config.json:ro \
  image-syncer:latest
```

需要挂载 `~/.docker/config.json` 以提供源仓库和 Harbor 的认证信息。可通过 Kubernetes CronJob 或 cron 定时执行。

## 项目结构

```
├── main.go                         # 入口
├── cmd/                            # CLI 命令（sync, check, list, delete）
├── internal/
│   ├── config/                     # 配置加载、校验、规则过滤、镜像引用解析
│   ├── registry/
│   │   ├── types.go                # SourceClient 接口、SourceTag、Harbor 类型
│   │   ├── client.go               # 源客户端工厂（NewSourceClient）
│   │   ├── containers_image.go     # 基于 containers/image 的统一源客户端
│   │   └── harbor.go               # Harbor API v2 客户端（含认证）
│   ├── syncer/                     # 同步引擎、标签过滤、镜像复制、删除分析
│   └── logger/                     # 日志输出
├── config.yaml.example             # 配置示例
└── Dockerfile                      # 多阶段构建
```
