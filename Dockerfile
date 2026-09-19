FROM golang:1.25-alpine3.22 AS builder
WORKDIR /app
COPY . .
ARG GITHUB_SHA
ARG VERSION
ARG TARGETARCH=amd64
RUN apk add --no-cache nodejs zstd && \
    case "${TARGETARCH}" in \
      amd64) node_asset="node_linux_amd64.zst" ;; \
      arm64) node_asset="node_linux_arm64.zst" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    zstd -f /usr/bin/node -o "assets/${node_asset}"
RUN echo "Building commit: ${GITHUB_SHA:0:7}" && \
    go mod download && \
    go build -ldflags="-s -w -X main.Version=${VERSION} -X main.CurrentCommit=${GITHUB_SHA:0:7}" -trimpath -o subs-check .

FROM alpine:3.22
WORKDIR /app
ENV TZ=Asia/Shanghai
RUN apk add --no-cache alpine-conf ca-certificates nodejs &&\
    /usr/sbin/setup-timezone -z Asia/Shanghai && \
    apk del alpine-conf && \
    rm -rf /var/cache/apk/* && \
    rm -rf /usr/bin/node
COPY --from=builder /app/subs-check /app/subs-check
CMD ["/app/subs-check"]
EXPOSE 8199
EXPOSE 8299
