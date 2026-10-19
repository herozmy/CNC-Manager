# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/
COPY VERSION ./VERSION
RUN VERSION="$(cat VERSION)" && \
    cd backend && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/CNC-Manager \
      ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata su-exec

WORKDIR /opt/CNC-Manager
COPY --from=backend /out/CNC-Manager /opt/CNC-Manager/CNC-Manager
COPY --from=frontend /src/frontend/dist /opt/CNC-Manager/web
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod 0755 /usr/local/bin/docker-entrypoint.sh && mkdir -p /data

EXPOSE 8080
ENV TZ=Asia/Shanghai \
    PUID=0 \
    PGID=0 \
    CNC_ADDR=0.0.0.0:8080 \
    CNC_DATA_DIR=/data \
    CNC_WEB_DIR=/opt/CNC-Manager/web \
    CNC_NO_BROWSER=1 \
    CNC_DEPLOYMENT=docker \
    CNC_MAX_UPLOAD_MB=64

VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/opt/CNC-Manager/CNC-Manager"]
