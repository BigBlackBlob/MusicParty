FROM node:22-alpine AS frontend-builder
WORKDIR /src/music-party-web
COPY music-party-web/package.json music-party-web/pnpm-lock.yaml music-party-web/pnpm-workspace.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY music-party-web/ ./
RUN pnpm build

FROM golang:1.26.5-alpine AS backend-builder
WORKDIR /src/backend-go
COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download
COPY backend-go/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/musicparty ./cmd/musicparty \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/dbsnapshot ./cmd/dbsnapshot \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/dbcheck ./cmd/dbcheck

FROM alpine:3.23

ARG OCI_SOURCE="https://github.com/BigBlackBlob/MusicParty"
ARG OCI_REVISION="unknown"
ARG OCI_CREATED="unknown"
ARG OCI_VERSION="dev"

LABEL org.opencontainers.image.source=$OCI_SOURCE \
      org.opencontainers.image.revision=$OCI_REVISION \
      org.opencontainers.image.created=$OCI_CREATED \
      org.opencontainers.image.version=$OCI_VERSION

WORKDIR /app

RUN apk add --no-cache ca-certificates ffmpeg python3 py3-pip tzdata wget \
    && pip3 install --no-cache-dir --break-system-packages -U yt-dlp \
    && addgroup -S -g 10001 appgroup \
    && adduser -S -D -H -u 10001 -G appgroup appuser \
    && mkdir -p /app/data /app/cached_media /app/static \
    && chown -R appuser:appgroup /app

COPY --from=backend-builder /out/musicparty /app/musicparty
COPY --from=backend-builder /out/dbsnapshot /app/dbsnapshot
COPY --from=backend-builder /out/dbcheck /app/dbcheck
COPY --from=frontend-builder /src/music-party-web/dist /app/static

ENV STATIC_PATH=/app/static
USER 10001:10001
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/actuator/health/readiness >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/musicparty"]
