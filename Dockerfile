# 基础镜像可通过 build ARG 覆盖（例如使用国内加速源），默认官方镜像。
ARG GO_IMAGE=golang:1.22-alpine
ARG RUNTIME_IMAGE=alpine:3.20

# ---------- 构建阶段 ----------
FROM ${GO_IMAGE} AS builder
WORKDIR /app

# 先复制 go.mod 以复用依赖层缓存（本项目零第三方依赖）
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/resparse ./cmd/server

# ---------- 运行阶段 ----------
FROM ${RUNTIME_IMAGE}

# ffmpeg 提供 ffprobe 可执行文件；tini 负责转发信号并回收 ffprobe 子进程
RUN apk add --no-cache ffmpeg tini

WORKDIR /app
ENV PORT=3000 \
    HOST=0.0.0.0 \
    FFPROBE_PATH=ffprobe

COPY --from=builder /out/resparse /app/resparse

EXPOSE 3000

# 依赖 /api/health：ffprobe 可用即视为健康
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/app/resparse", "healthcheck"]

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/app/resparse"]
