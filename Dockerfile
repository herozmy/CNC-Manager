FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN mkdir -p ../backend/internal/web && npm run build

FROM golang:1.27.1-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /src/backend/internal/web/dist ./internal/web/dist
COPY VERSION /src/VERSION
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=$(tr -d '[:space:]' < /src/VERSION)" \
    -o /out/cnc-manager ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
    -o /out/migrate-v1 ./cmd/migrate-v1

FROM alpine:3.22
ARG VCS_REF=unknown
LABEL org.opencontainers.image.title="CNC-Manager v2" \
      org.opencontainers.image.source="https://github.com/herozmy/CNC-Manager" \
      org.opencontainers.image.revision=$VCS_REF
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 app && adduser -D -u 1000 -G app app \
    && mkdir /data && chown app:app /data
COPY --from=build /out/cnc-manager /usr/local/bin/cnc-manager
COPY --from=build /out/migrate-v1 /usr/local/bin/migrate-v1
WORKDIR /data
USER 1000:1000
ENV CNC_ADDR=0.0.0.0:8080 CNC_DATA_DIR=/data TZ=Asia/Shanghai
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:8080/api/meta >/dev/null || exit 1
ENTRYPOINT ["cnc-manager"]
