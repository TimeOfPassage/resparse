#!/usr/bin/env sh
#
# 冒烟测试：构建二进制并验证接口行为。
#
# 无需真实 R2 凭据：未配置时解析接口应返回结构化的 config_missing 错误，
# 缺 key 时返回 400 invalid_key；未安装 ffmpeg 时 /api/health 返回 503，
# 本脚本仍视为通过。
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${SMOKE_PORT:-38080}"
BIN_DIR="$(mktemp -d)"
BIN="$BIN_DIR/resparse"

cleanup() {
  if [ -n "$PID" ]; then kill "$PID" 2>/dev/null || true; fi
  rm -rf "$BIN_DIR"
}
trap cleanup EXIT

echo "==> 构建"
(cd "$ROOT" && go build -o "$BIN" ./cmd/server)

echo "==> 启动服务（127.0.0.1:$PORT，无 R2 凭据）"
PORT="$PORT" HOST=127.0.0.1 "$BIN" >"$BIN_DIR/server.log" 2>&1 &
PID=$!

echo "==> 等待就绪"
i=0
while [ "$i" -lt 50 ]; do
  if curl -sS -o /dev/null "http://127.0.0.1:$PORT/api/health" 2>/dev/null; then
    break
  fi
  i=$((i + 1))
  sleep 0.1
done

echo
echo "==> GET /api/health"
curl -sS -w "\nHTTP %{http_code}\n" "http://127.0.0.1:$PORT/api/health" || true

echo
echo "==> GET /api/metadata（缺 key，期望 400 invalid_key）"
curl -sS -w "\nHTTP %{http_code}\n" "http://127.0.0.1:$PORT/api/metadata" || true

echo
echo "==> GET /api/metadata?key=demo.mp4（无 R2 凭据，期望 500 config_missing）"
curl -sS -w "\nHTTP %{http_code}\n" "http://127.0.0.1:$PORT/api/metadata?key=demo.mp4" || true

echo
echo "==> GET /（演示页应返回 200 HTML）"
curl -sS -o /dev/null -w "HTTP %{http_code} %{content_type}\n" "http://127.0.0.1:$PORT/" || true

echo
echo "==> 服务日志"
cat "$BIN_DIR/server.log"

echo
echo "==> 冒烟测试完成"
