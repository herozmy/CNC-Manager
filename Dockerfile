FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN mkdir -p ../backend/internal/web && npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /src/backend/internal/web/dist ./internal/web/dist
COPY VERSION /src/VERSION
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=$(tr -d '[:space:]' < /src/VERSION)" \
    -o /out/cnc-manager ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 1000 app && mkdir /data && chown app /data
COPY --from=build /out/cnc-manager /usr/local/bin/cnc-manager
USER app
ENV CNC_ADDR=0.0.0.0:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/api/meta >/dev/null || exit 1
CMD ["cnc-manager"]
