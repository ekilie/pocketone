# The image `pocketone desktop build` runs in: Go, a pocketone built from this checkout, and the desktop app module
# with every dependency vendored, so builds need nothing else and run offline. The build context is the checkout:
#
#   docker build -f ext/desktop/image/Dockerfile -t pocketone-build:dev .
#
# (`pocketone desktop image` runs that; a development pocketone runs it on first use.)

FROM golang:1.27-bookworm AS prepare
ENV GOTOOLCHAIN=local CGO_ENABLED=0
COPY . /src
RUN --mount=type=cache,target=/root/go/pkg/mod \
    cd /src/examples/pocketone && go build -trimpath -o /usr/local/bin/pocketone .
RUN --mount=type=cache,target=/root/go/pkg/mod \
    pocketone desktop prepare --source /src /opt/pocketone-build

FROM golang:1.27-bookworm
COPY --from=prepare /usr/local/bin/pocketone /usr/local/bin/pocketone
COPY --from=prepare /opt/pocketone-build /opt/pocketone-build
# offline, against the vendored module; the build cache lives in a volume (pocketone-build-cache) between runs
ENV GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-mod=vendor GOPROXY=off GOSUMDB=off \
    P1_DESKTOP_TEMPLATE=/opt/pocketone-build GOCACHE=/cache/go-build HOME=/tmp
RUN mkdir -m 1777 /cache && chmod -R a+rwX /opt/pocketone-build
WORKDIR /app
ENTRYPOINT ["pocketone"]
