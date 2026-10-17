# syntax=docker/dockerfile:1
# Generated compiler helpers and UI assets are committed and embedded by Go.
ARG GO_IMAGE=golang:1.25-bookworm
ARG NODE_IMAGE=node:22-bookworm-slim
FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/seamgraph ./cmd/seamgraph

FROM ${NODE_IMAGE} AS runtime
USER root
ARG APP_UID=1000
ARG APP_GID=1000
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 python3-venv git openssh-client ca-certificates build-essential pkg-config \
    && rm -rf /var/lib/apt/lists/*
# Keep Go in the runtime: go/packages invokes the Go toolchain during analysis.
COPY --from=build /usr/local/go /usr/local/go
COPY --from=build /out/seamgraph /usr/local/bin/seamgraph
ENV PATH="/usr/local/go/bin:${PATH}" \
    XDG_CACHE_HOME=/data/cache \
    GOPATH=/data/go \
    GOMODCACHE=/data/go-mod \
    GOCACHE=/data/go-build \
    NPM_CONFIG_CACHE=/data/npm \
    PIP_CACHE_DIR=/data/pip
RUN if [ "$APP_GID" != 1000 ]; then groupmod -g "$APP_GID" node; fi \
    && if [ "$APP_UID" != 1000 ]; then usermod -u "$APP_UID" node; fi \
    && mkdir -p /data /project /home/node/.ssh && chown -R node:node /data /project /home/node/.ssh
USER node
WORKDIR /project
# Fail the image build if any required external runtime is missing or too old.
RUN seamgraph doctor
EXPOSE 8765
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s CMD ["node", "-e", "require('http').get('http://127.0.0.1:8765/',r=>{r.resume();process.exit(r.statusCode===200?0:1)}).on('error',()=>process.exit(1))"]
ENTRYPOINT ["seamgraph"]
CMD ["doctor"]
