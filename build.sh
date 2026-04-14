#!/bin/bash
set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <tag> [tag2] [tag3] ..."
    echo "Example: $0 v1.0.0"
    echo "Example: $0 v1.0.0 latest"
    exit 1
fi

TAGS=("$@")

REGISTRY="hub.wiolfi.net:23333/wolfbolin/docker-image-sync"

ARCH=$(uname -m)
case "${ARCH}" in
  x86_64)  ARCH_TAG="amd64" ;;
  aarch64) ARCH_TAG="arm64" ;;
  *)       echo "不支持的架构: ${ARCH}"; exit 1 ;;
esac

for TAG in "${TAGS[@]}"; do
  IMAGE="${REGISTRY}:${TAG}"
  ARCH_IMAGE="${IMAGE}-${ARCH_TAG}"

  echo ""
  echo "构建镜像: ${ARCH_IMAGE} (${ARCH})"
  podman build --tag "${ARCH_IMAGE}" .

  echo ""
  echo "创建镜像 manifest: ${IMAGE}"
  if podman manifest inspect "${IMAGE}" &>/dev/null; then
    podman manifest rm "${IMAGE}"
  fi
  podman manifest create "${IMAGE}"
  podman manifest add "${IMAGE}" "containers-storage:${ARCH_IMAGE}"

  echo ""
  echo "推送镜像 manifest: ${IMAGE}"
  podman manifest push "${IMAGE}"
done

echo ""
echo "完成: ${IMAGE}"