# docker-image-sync

docker-image-sync 是一个将公共容器镜像仓库中的镜像同步到私有 Harbor/OCI 仓库的命令行工具。当前实现基于 `containers/image` 直接完成标签查询、镜像复制和标签删除，不依赖本机安装 `skopeo`。

典型场景是在内网或受限网络环境中，按规则从 Docker Hub、registry.k8s.io、Quay、GHCR、阿里云 ACR 等公共仓库拉取镜像，并推送到内部 Harbor 供集群或业务系统使用。

## 当前能力

- 通过 `sync` 同步镜像标签，默认只复制目标端不存在的标签。
- 通过 `sync --dry-run` 预览同步计划；`check` 命令已移除。
- 通过 `sync -f/--force` 对已存在标签比较 digest，发现差异后重新同步。
- 通过 `list` 查看目标端标签；`list --online` 同时查看 source 和 target 两侧标签。
- 通过 `delete` 删除目标端冗余标签；支持 `--dry-run` 预览。
- 每条规则可独立决定 source 侧是否使用代理。
- 镜像复制使用 `copy.CopyAllImages`，支持多架构 manifest list。
- 认证由 `containers/image` 按标准容器认证配置读取，通常来自 `~/.docker/config.json`。

## 技术栈

- Go 1.25+
- CLI: `github.com/spf13/cobra`
- 镜像操作: `github.com/containers/image/v5`
- 配置解析: `gopkg.in/yaml.v3`

构建时必须使用 `containers_image_openpgp` tag，并建议关闭 CGO：

```bash
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o bin/image-syncer .
```

## 配置

当前配置模型由 `internal/cfg` 定义。核心字段是顶层 `proxy`、`retry` 和 `rules`，规则目标字段名是 `target`。

```yaml
proxy: "http://10.10.30.34:12821"

retry:
  times: 3
  interval: 5s

rules:
  - name: "ubuntu"
    source: "docker.io/library/ubuntu"
    target: "hub.example.com:8443/docker.io/ubuntu"
    proxy: true
    tags: ["22.04", "24.04"]

  - name: "alpine"
    source: "docker.io/library/alpine"
    target: "hub.example.com:8443/docker.io/alpine"
    proxy: true
    tag_regex: '^\d+\.\d+(\.\d+)?$'
```

字段说明：

| 字段 | 位置 | 说明 |
| --- | --- | --- |
| `proxy` | 顶层 | source 侧代理地址。仅当规则 `proxy: true` 时设置到 source client。 |
| `retry.times` | 顶层 | copy/delete 失败后的最大尝试次数；小于等于 0 时重置为 3。 |
| `retry.interval` | 顶层 | 重试间隔配置；小于等于 0 时重置为 5s。 |
| `rules[].name` | 规则 | 规则名称，用于 `-r/--rule` 过滤；非空时必须唯一。 |
| `rules[].source` | 规则 | 源镜像仓库，例如 `docker.io/library/alpine`。 |
| `rules[].target` | 规则 | 目标镜像仓库，例如 `hub.example.com:8443/docker.io/alpine`。 |
| `rules[].proxy` | 规则 | 是否对 source 侧查询和拉取启用顶层代理。 |
| `rules[].tags` | 规则 | 精确匹配的标签列表。 |
| `rules[].tag_regex` | 规则 | 标签正则表达式；配置时会进行正则合法性校验。 |

镜像引用使用 `/` 分段解析。第一段如果是域名或 `host:port`，会识别为 registry；否则会作为 project/name 解析。

配置文件查找顺序：

1. 命令行 `-c/--config` 指定的路径。
2. 环境变量 `SYNC_DOCKER_CONFIG`。
3. `$HOME/.config/docker-image-sync/config.yaml`。
4. 当前工作目录下的 `config.yaml`。

自动化验证请使用 `./config-test.yaml`，不要使用本地 `./config.yaml`。

## 命令

所有业务命令都支持 `-c/--config` 和 `-r/--rule`。`--rule` 接收逗号分隔的规则名称，不传则处理全部规则。

### sync

执行同步。流程为：读取 source/target 标签，按规则筛选 source 标签，计算 `Sync/Over/Diff/Same`，再复制 `Sync` 和 `Diff` 标签。

```bash
image-syncer sync -c config-test.yaml
image-syncer sync -c config-test.yaml -r ubuntu,alpine
image-syncer sync -c config-test.yaml -r alpine --dry-run
image-syncer sync -c config-test.yaml -r alpine --dry-run -f
```

参数：

| 参数 | 说明 |
| --- | --- |
| `--dry-run` | 只执行标签查询和计划输出，不复制镜像。 |
| `-f, --force` | 对 `Same` 标签比较 source/target digest；digest 不同时放入 `Diff` 并在非 dry-run 模式下重新复制。 |

输出分组含义：

| 分组 | 含义 |
| --- | --- |
| `Sync` | source 匹配规则且 target 不存在，待新增。 |
| `Over` | target 存在但不在本次 source 匹配结果中。 |
| `Diff` | source/target 同名标签 digest 不同，需更新。 |
| `Same` | source/target 同名标签一致或未开启 digest 比较时已存在。 |

### list

列出标签。默认只查询 target 侧；使用 `--online` 时会额外查询 source 侧。

```bash
image-syncer list -c config-test.yaml
image-syncer list -c config-test.yaml -r alpine
image-syncer list -c config-test.yaml -r alpine --online
```

`list --online` 查询 source 时会遵循规则里的 `proxy` 设置。

### delete

删除 target 侧冗余标签。

```bash
image-syncer delete -c config-test.yaml --dry-run
image-syncer delete -c config-test.yaml -r ubuntu --dry-run
image-syncer delete -c config-test.yaml -r ubuntu
image-syncer delete -c config-test.yaml -r ubuntu --online --dry-run
```

当前删除流程会先计算待删除标签，再由 `ExecuteDelete` 删除 `Over` 分组。建议先使用 `--dry-run` 查看输出。

## 构建与验证

```bash
# 构建本地二进制
mkdir -p bin
CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o bin/image-syncer .

# 静态检查
CGO_ENABLED=0 go vet -tags "containers_image_openpgp" ./...

# 单元测试
CGO_ENABLED=0 go test -tags "containers_image_openpgp" ./cmd ./internal/sync

# 查看命令
./bin/image-syncer --help
./bin/image-syncer sync --help
./bin/image-syncer list --help
./bin/image-syncer delete --help
```

`internal/hub/client_test.go` 会访问外部 registry，离线环境或受限网络中不建议直接跑全量 `go test ./...`。

## Docker

仓库包含多阶段 `Dockerfile`，用于构建容器镜像：

```bash
docker build -t docker-image-sync:latest .
```

运行时通常需要挂载配置文件和 Docker 认证文件：

```bash
docker run --rm \
  -v /path/to/config.yaml:/etc/docker-image-sync/config.yaml:ro \
  -v ~/.docker/config.json:/root/.docker/config.json:ro \
  docker-image-sync:latest sync -c /etc/docker-image-sync/config.yaml
```

## 项目结构

```text
├── main.go
├── cmd/
│   ├── root.go        # 根命令和统一输出函数
│   ├── sync.go        # sync 子命令，含 --dry-run 和 --force
│   ├── list.go        # list 子命令，含 --online
│   └── delete.go      # delete 子命令，含 --dry-run 和 --online
├── internal/
│   ├── cfg/           # 配置结构、加载、校验、规则过滤
│   ├── hub/           # containers/image 客户端和镜像引用解析
│   ├── sync/          # 标签分析、镜像复制、删除执行
│   └── logger/        # 卡片输出和标签分组输出
├── docs/
├── config-test.yaml
├── config.yaml.example
├── Dockerfile
└── go.mod / go.sum
```

## 注意事项

- 当前没有 `check` 子命令；使用 `sync --dry-run` 预览同步计划。
- source 侧代理只在规则 `proxy: true` 时设置，target 侧不设置代理。
- 镜像复制当前设置 `PreserveDigests: false`，以提高 schema1 等旧镜像兼容性。
- `filterTags` 会同时接受 `tags` 和 `tag_regex` 的匹配结果；配置层目前不强制二选一。
