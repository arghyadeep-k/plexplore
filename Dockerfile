# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=1
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/plexplore-server ./cmd/server
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/plexplore-migrate ./cmd/migrate

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite tzdata

COPY --from=build /out/plexplore-server /usr/local/bin/plexplore-server
COPY --from=build /out/plexplore-migrate /usr/local/bin/plexplore-migrate
COPY migrations ./migrations
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

RUN addgroup -S plexplore && adduser -S -G plexplore plexplore
RUN mkdir -p /data/spool && chown -R plexplore:plexplore /app /data

USER plexplore

ENV APP_DEPLOYMENT_MODE=production
ENV APP_HTTP_LISTEN_ADDR=0.0.0.0:8080
ENV APP_SQLITE_PATH=/data/plexplore.db
ENV APP_SPOOL_DIR=/data/spool
ENV APP_MIGRATIONS_DIR=/app/migrations
ENV APP_COOKIE_SECURE_MODE=always
ENV APP_TRUST_PROXY_HEADERS=false
ENV APP_EXPECT_TLS_TERMINATION=true
ENV APP_ALLOW_INSECURE_HTTP=false
ENV APP_MAP_TILE_MODE=none
ENV APP_MAP_TILE_URL_TEMPLATE=https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png
ENV APP_MAP_TILE_ATTRIBUTION='&copy; OpenStreetMap contributors'

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
