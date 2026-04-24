# DEV Dockerfile for the scheduler. Go-only build — no frontend step, no
# go:embed of the SPA. This is what docker compose builds; Vite serves the
# frontend at :5173 in dev and proxies /api/* to this image at :8080.
#
# Production Dockerfile (with frontend embedded) lives at the repo root. See
# docs/DEPLOY.md for the Railway deploy model.

FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/scheduler .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/scheduler /scheduler
ENTRYPOINT ["/scheduler"]
