# syntax=docker/dockerfile:1

# --- Frontend production build ------------------------------------------------
FROM node:22-bookworm AS frontend
WORKDIR /src
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN cd web && pnpm install --frozen-lockfile
COPY web/ ./web/
RUN cd web && pnpm build

# --- Go compile with embedded assets ------------------------------------------
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- Minimal runtime ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

# Migrations stay on disk next to the binary (not frontend assets — those are
# embedded). The nonroot image runs as uid 65532.
COPY --from=build /out/server /app/server
COPY db/migrations /app/db/migrations

ENV MIGRATIONS_DIR=/app/db/migrations
ENV PORT=8080

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
# Default: serve traffic. Migrations: docker run … <image> migrate up
CMD ["serve"]
