# Build stage
FROM golang:1.26-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS builder

WORKDIR /app

# 通过构建参数接收敏感信息
ARG GOPRIVATE_ARG
ARG GOPROXY_ARG=https://goproxy.cn,direct
ARG GOSUMDB_ARG=off
ARG APK_MIRROR_ARG

# 设置Go环境变量
ENV GOPRIVATE=${GOPRIVATE_ARG}
ENV GOPROXY=${GOPROXY_ARG}
ENV GOSUMDB=${GOSUMDB_ARG}

# Install dependencies
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y git build-essential libsqlite3-dev

# Install migrate tool
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/download cmd/download
RUN --mount=type=cache,target=/go/pkg/mod go run cmd/download/duckdb/duckdb.go
COPY . .

# Get version and commit info for build injection
ARG VERSION_ARG
ARG COMMIT_ID_ARG
ARG BUILD_TIME_ARG
ARG GO_VERSION_ARG

# Set build-time variables
ENV VERSION=${VERSION_ARG}
ENV COMMIT_ID=${COMMIT_ID_ARG}
ENV BUILD_TIME=${BUILD_TIME_ARG}
ENV GO_VERSION=${GO_VERSION_ARG}

# Build the application with version info
RUN --mount=type=cache,target=/go/pkg/mod make build-prod
# The Go module cache is mounted only while building, so copy Jieba's runtime
# dictionaries into the image explicitly.  The application initializes Jieba
# during startup and cannot use the zero-byte cache placeholders otherwise.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download github.com/yanyiwu/gojieba@v1.4.7 && \
    mkdir -p /app/jieba-dict && \
    cp /go/pkg/mod/github.com/yanyiwu/gojieba@v1.4.7/deps/cppjieba/dict/jieba.dict.utf8 \
       /go/pkg/mod/github.com/yanyiwu/gojieba@v1.4.7/deps/cppjieba/dict/hmm_model.utf8 \
       /go/pkg/mod/github.com/yanyiwu/gojieba@v1.4.7/deps/cppjieba/dict/user.dict.utf8 \
       /go/pkg/mod/github.com/yanyiwu/gojieba@v1.4.7/deps/cppjieba/dict/idf.utf8 \
       /go/pkg/mod/github.com/yanyiwu/gojieba@v1.4.7/deps/cppjieba/dict/stop_words.utf8 \
       /app/jieba-dict/

# Final stage
FROM debian:12.12-slim@sha256:d5d3f9c23164ea16f31852f95bd5959aad1c5e854332fe00f7b3a20fcc9f635c

WORKDIR /app

ARG APK_MIRROR_ARG

# Create a non-root user first
RUN useradd -m -s /bin/bash appuser

# The slim runtime image may not contain a CA bundle yet. Reuse the verified
# bundle from the build stage so HTTPS mirrors work before apt installs its own.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Switch to the configured Debian mirror before the first package operation.
# Debian's base image already includes the keys needed to validate its package
# indexes, so this keeps every APT stage on the same reachable mirror.
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Install the remaining runtime packages from the same source.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential postgresql-client default-mysql-client tzdata sed curl bash vim wget \
        libsqlite3-0 \
        python3 python3-pip python3-dev libffi-dev libssl-dev \
        nodejs npm \
        gosu \
        ffmpeg && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --break-system-packages --upgrade pip setuptools wheel uv && \
    mkdir -p /home/appuser/.local/bin && \
    chown -R appuser:appuser /home/appuser

# Create data directories and set permissions
RUN mkdir -p /data/files && \
    chown -R appuser:appuser /app /data/files

# Copy migrate tool from builder stage
COPY --from=builder /go/bin/migrate /usr/local/bin/
COPY --from=builder /app/jieba-dict ./jieba-dict

# Copy the binary from the builder stage
COPY --from=builder /app/config ./config
COPY --from=builder /app/scripts ./scripts
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/dataset/samples ./dataset/samples
COPY --from=builder /app/skills/preloaded ./skills/preloaded
# Keep a read-only backup so bind-mount cannot erase built-in skills
COPY --from=builder /app/skills/preloaded ./skills/_builtin
COPY --from=builder /root/.duckdb /home/appuser/.duckdb
COPY --from=builder /app/FMind .

# Copy and make entrypoint script executable
COPY --from=builder /app/scripts/docker-entrypoint.sh ./scripts/docker-entrypoint.sh

# Make scripts executable
RUN chmod +x ./scripts/*.sh

# Expose ports
EXPOSE 8080

ENV JIEBA_DICT_DIR=/app/jieba-dict


ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
CMD ["./FMind"]
