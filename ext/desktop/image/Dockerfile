# The image `pocketone desktop build` runs in: Go, a pocketone built from this checkout, and the desktop app module
# with every dependency vendored, so builds need nothing else and run offline. The build context is the checkout:
#
#   docker build -f ext/desktop/image/Dockerfile -t pocketone-build:dev .
#
# (`pocketone desktop image` runs that; a development pocketone runs it on first use.) Every release tag publishes
# it for linux/amd64 and linux/arm64 as ghcr.io/pocketonehq/pocketone-build:<tag> (.github/workflows/build-image.yml).

# The prepare stage runs on the build machine's own architecture and cross-compiles the image's pocketone, so a
# multi-arch build (linux/amd64 and linux/arm64) never compiles Go under emulation. vendor/ is the same for every
# architecture.
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS prepare
ARG TARGETOS TARGETARCH
ENV GOTOOLCHAIN=local CGO_ENABLED=0
COPY . /src
WORKDIR /src/examples/pocketone
RUN --mount=type=cache,target=/root/go/pkg/mod \
    go build -trimpath -o /usr/local/bin/pocketone-host . && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/pocketone .
RUN --mount=type=cache,target=/root/go/pkg/mod \
    pocketone-host desktop prepare --source /src /opt/pocketone-build

FROM golang:1.27-bookworm
COPY --from=prepare /out/pocketone /usr/local/bin/pocketone
COPY --from=prepare /opt/pocketone-build /opt/pocketone-build
# offline, against the vendored module; the build cache lives in a volume (pocketone-build-cache) between runs
ENV GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-mod=vendor GOPROXY=off GOSUMDB=off \
    P1_DESKTOP_TEMPLATE=/opt/pocketone-build GOCACHE=/cache/go-build HOME=/tmp
RUN mkdir -m 1777 /cache && chmod -R a+rwX /opt/pocketone-build
WORKDIR /app
ENTRYPOINT ["pocketone"]
