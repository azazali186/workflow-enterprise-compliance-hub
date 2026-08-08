# --- build stage ---
# go.mod requires Go 1.25; the toolchain image must match or the module
# refuses to compile.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /bin/server /app/server
# /app is the working directory: the server writes routes.json there during
# the startup permission sync, so it must be writable by the non-root user.
RUN chown -R app:app /app
USER app
EXPOSE 8080

# Liveness probe against the public /health endpoint (busybox wget).
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server"]
