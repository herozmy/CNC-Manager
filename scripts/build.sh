#!/usr/bin/env bash
# 构建前端并编译内嵌前端的 Linux amd64 单文件服务。产物：.local/release/
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="$(tr -d '[:space:]' < VERSION)"
(cd frontend && npm ci && npm run typecheck && npm test && npm run build)
(cd backend && go test ./...)
mkdir -p .local/release
python3 -m unittest discover -s scripts -p 'test_*.py'
install -m 700 scripts/reset-password.py .local/release/cnc-reset-password
(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
  -o "../.local/release/cnc-manager" ./cmd/server)
echo "已生成 .local/release/cnc-manager (${VERSION})"
echo "已打包 .local/release/cnc-reset-password（需要 Python 3）"
