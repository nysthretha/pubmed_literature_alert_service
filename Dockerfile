# Production multi-stage build for the scheduler service.
#
# This Dockerfile is Railway's build target for the scheduler. It builds the
# React SPA, compiles the Go binary with the SPA embedded via go:embed, and
# copies the result into a distroless runtime.
#
# Dev is unaffected: docker compose continues to build from scheduler/Dockerfile
# (Go-only, no frontend step), since Vite serves the SPA separately at :5173.

# ------------------------------------------------------------------ stage: web
FROM node:20-alpine AS webbuilder
WORKDIR /build/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build
# Output: /build/web/dist

# ------------------------------------------------------------------- stage: go
FROM golang:1.25-alpine AS gobuilder
WORKDIR /build/scheduler

COPY scheduler/go.mod scheduler/go.sum ./
RUN go mod download

COPY scheduler/ ./

# Place the built frontend where embed_prod.go expects it: the `//go:embed
# all:web/dist` directive resolves relative to the file at scheduler/embed_prod.go.
COPY --from=webbuilder /build/web/dist/ ./web/dist/

# The `production` tag activates embed_prod.go; without it the binary compiles
# against embed_dev.go and ships an empty embed FS.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -tags=production \
    -ldflags="-s -w" \
    -o /out/scheduler .

# -------------------------------------------------------------- stage: runtime
FROM gcr.io/distroless/static-debian12:nonroot

# Distroless static has no package manager, no shell, and runs as nonroot by
# default. CA certs for TLS (Brevo SMTP, NCBI E-utilities) are included.
COPY --from=gobuilder /out/scheduler /scheduler

EXPOSE 8080
ENTRYPOINT ["/scheduler"]
