# syntax=docker/dockerfile:1

# NOTE: The build stage intentionally runs at TARGETPLATFORM (not BUILDPLATFORM).
# go-sqlite3 requires CGO, and there is no cross C toolchain installed here, so a
# BUILDPLATFORM-pinned stage cannot cross-compile for the Pi (arm64/armhf). Running
# at TARGETPLATFORM lets the native compiler do the work (under QEMU emulation when
# building for ARM from an amd64 host, e.g. `docker buildx build --platform linux/arm64`).
FROM golang:1.24-alpine AS build
WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
RUN go build -o /out/exploripi-server ./cmd/server
RUN go build -o /out/exploripi-migrate ./cmd/migrate

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite tzdata

COPY --from=build /out/exploripi-server /usr/local/bin/exploripi-server
COPY --from=build /out/exploripi-migrate /usr/local/bin/exploripi-migrate
COPY migrations ./migrations
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

RUN addgroup -S exploripi && adduser -S -G exploripi exploripi
RUN mkdir -p /data/spool && chown -R exploripi:exploripi /app /data

USER exploripi

ENV APP_DEPLOYMENT_MODE=production
ENV APP_HTTP_LISTEN_ADDR=0.0.0.0:8080
ENV APP_SQLITE_PATH=/data/exploripi.db
ENV APP_SPOOL_DIR=/data/spool
ENV APP_MIGRATIONS_DIR=/app/migrations
ENV APP_COOKIE_SECURE_MODE=always
ENV APP_TRUST_PROXY_HEADERS=false
ENV APP_EXPECT_TLS_TERMINATION=true
ENV APP_ALLOW_INSECURE_HTTP=false
ENV APP_MAP_TILE_MODE=none
ENV APP_MAP_TILE_URL_TEMPLATE=https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png
ENV APP_MAP_TILE_ATTRIBUTION="&copy; OpenStreetMap contributors"

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
