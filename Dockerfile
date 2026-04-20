# 构建阶段
FROM hub.wiolfi.net:23333/docker.io/golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -tags "containers_image_openpgp" -o image-syncer .

# 运行阶段
FROM hub.wiolfi.net:23333/docker.io/alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/image-syncer /usr/local/bin/image-syncer

# 使用 ENTRYPOINT 允许灵活的子命令调用
ENTRYPOINT ["/usr/local/bin/image-syncer"]
# 默认参数（可被覆盖）
CMD ["sync"]
