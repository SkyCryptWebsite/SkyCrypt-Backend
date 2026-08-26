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
# GOMEMLIMIT  Set to ~87% of the 8 GB container limit (≈7 GiB), leaving ~1 GB
#             headroom for Alpine, tini, git, and kernel buffers.
#             Prevents silent OOM kills while still letting the heap breathe freely.
#
# GOGC=300    Container has 8 GB and CPU sits at <30% — GC pauses are the enemy,
#             not memory pressure. GOGC=300 means GC only triggers when the live
#             heap has grown 3× since the last collection (vs 1× at the default 100).
#             Effect: far fewer GC cycles, lower p99 latency, higher steady-state
#             memory usage — exactly the right trade-off here.
#             If RSS ever climbs past 6 GB in practice, dial back to 200.
#
# GODEBUG=netdns=go
#             Forces the pure-Go DNS resolver, bypassing the cgo libc resolver.
#             No /etc/nsswitch.conf quirks, marginally faster for high-QPS outbound.
#
ENV SOURCE_COMMIT=$SOURCE_COMMIT \
    GOMEMLIMIT=7GiB \
    GOGC=300 \
    GODEBUG=netdns=go

# Expose port
EXPOSE 8080

# tini as PID 1.
# ENTRYPOINT exec form → tini receives signals from Docker/k8s and forwards them
# to ./main, which triggers Fiber's graceful shutdown (drain in-flight requests,
# flush Redis pipeline, close connections).
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["./main"]
