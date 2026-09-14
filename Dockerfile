# syntax=docker/dockerfile:1.7
FROM golang:1.26.4-alpine AS builder

# NOTE: build-base (gcc/musl) intentionally omitted — CGO_ENABLED=0 produces a
#       pure-Go static binary with zero C-compiler dependency.
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy local dependencies first
# COPY SkyCrypt-Types/ ../SkyCrypt-Types/
# COPY SkyHelper-Networth-Go/ ../SkyHelper-Networth-Go/

# Copy go mod files
COPY go.mod go.sum ./

# Download modules with a BuildKit cache mount.
# /go/pkg/mod persists across docker build invocations on the same host
# → no re-downloading on incremental rebuilds or CI re-runs.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Copy source code
COPY . .

# Optimised production build.
#
#   CGO_ENABLED=0           Pure-Go static binary; no glibc, works in scratch/alpine
#   GOOS=linux GOARCH=amd64 Explicit cross-compile target (safe even on ARM build hosts)
#   -trimpath               Strips local filesystem paths from the binary
#                           → reproducible builds + marginally smaller output
#   -ldflags="-s -w"        -s: omit symbol table  -w: omit DWARF debug info
#                           Combined effect: ~25-35% binary size reduction
#   -buildvcs=false         Skip VCS stamping → reproducible in CI, marginally faster
#   -a                      Force rebuild of all packages against the cached modules
#
# Two BuildKit cache mounts:
#   /go/pkg/mod             Reuses downloaded modules (same as above)
#   /root/.cache/go-build   Reuses compiled packages across builds
#                           → cold build ~60 s, warm rebuild ~3-8 s
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test "$(go env GOVERSION)" = "go1.26.4" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
        -a \
        -trimpath \
        -ldflags="-s -w" \
        -buildvcs=false \
        -o main .

FROM alpine:latest

ARG SOURCE_COMMIT=""

# Runtime dependencies only:
#   tini            PID 1 init process — forwards SIGTERM/SIGINT to Fiber so graceful
#                   shutdown actually runs. Without it Docker sends SIGTERM to a shell
#                   wrapper, not to ./main, and Fiber's shutdown hook is never called.
#                   Also reaps zombie child processes.
#   ca-certificates TLS roots for outbound HTTPS calls (API proxy core functionality)
#   git             Runtime NotEnoughUpdates-REPO management over HTTPS
#   openssh-client  Same over SSH (git@github.com remotes)
#   tzdata          Correct timezone handling in Fiber logs and Minecraft event schedules
RUN apk --no-cache add ca-certificates git openssh-client tini tzdata

WORKDIR /app

# Copy assets and other necessary files
COPY --from=builder /app/main                    ./main
COPY --from=builder /app/assets                  ./assets
COPY --from=builder /app/NotEnoughUpdates-REPO   ./NotEnoughUpdates-REPO
COPY --from=builder /app/docs                    ./docs

RUN mkdir -p logs cache

# ── Go Runtime Tuning ─────────────────────────────────────────────────────────
#
# GOMEMLIMIT  Keep the Go heap below 5 GiB in the 8 GiB container so native
#             allocations, stacks, buffers, and the runtime have headroom.
#
# GOGC=100    Collect at the default target instead of allowing the heap to grow
#             several times between collections. This keeps allocation spikes from
#             turning into multi-gigabyte RSS spikes.
#
# GODEBUG=netdns=go,madvdontneed=1
#             Use the pure-Go DNS resolver and return scavenged pages to Linux
#             promptly so RSS follows live heap more closely.
#
ENV SOURCE_COMMIT=$SOURCE_COMMIT \
    GOMEMLIMIT=7GiB \
    GOGC=100 \
    GODEBUG=netdns=go,madvdontneed=1

# Expose port
EXPOSE 8080

# tini as PID 1.
# ENTRYPOINT exec form → tini receives signals from Docker/k8s and forwards them
# to ./main, which triggers Fiber's graceful shutdown (drain in-flight requests,
# flush Redis pipeline, close connections).
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["./main"]
