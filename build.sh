#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo none)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)}"
GO_VERSION="${GO_VERSION:-$(go version 2>/dev/null | awk '{print $3}' || echo unknown)}"
PLATFORM="${PLATFORM:-$(go env GOOS)/$(go env GOARCH)}"

BINARY="cs-cloud"
CMD="./cmd/cs-cloud"

case "${1:-build}" in
  build)
    mkdir -p bin
    swag init -g cmd/cs-cloud/main.go -o internal/localserver/apidocs --parseDependency --parseInternal 2>/dev/null || true
    go build -trimpath -ldflags "\
      -s -w \
      -X cs-cloud/internal/version.Version=${VERSION} \
      -X cs-cloud/internal/version.Commit=${COMMIT} \
      -X cs-cloud/internal/version.BuildTime=${BUILD_TIME} \
      -X cs-cloud/internal/version.GoVersion=${GO_VERSION} \
      -X cs-cloud/internal/version.Platform=${PLATFORM}" \
      -o "bin/${BINARY}" "${CMD}"
    echo "built bin/${BINARY} (${VERSION})"
    ;;
  test)
    go test ./...
    ;;
  lint)
    golangci-lint run ./...
    ;;
  vet)
    go vet ./...
    ;;
  fmt)
    gofmt -l -s -w .
    ;;
  clean)
    rm -rf bin/ dist/ build/
    ;;
  *)
    echo "Usage: $0 {build|test|lint|vet|fmt|clean}"
    exit 1
    ;;
esac
