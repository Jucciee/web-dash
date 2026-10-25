# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24.22.1-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY tokens.css /build/tokens.css
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.10-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /dashboard ./cmd/dashboard
RUN mkdir -p /data && chown 65532:65532 /data && chmod 700 /data

FROM scratch
WORKDIR /app
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend /dashboard /dashboard
COPY --from=backend --chown=65532:65532 /data /data
COPY --from=frontend /build/web/dist /app/web
ENV DASHBOARD_ADDR=:8080 DASHBOARD_DATA_DIR=/data DASHBOARD_STATIC_DIR=/app/web
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/dashboard"]
