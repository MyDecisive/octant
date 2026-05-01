# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25
ARG OCTANT_UI_IMAGE=ghcr.io/mydecisive/octant-ui:latest

FROM ${OCTANT_UI_IMAGE} as ui-builder

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS binary-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY --link go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY --link . .
COPY --from=ui-builder /usr/share/nginx/html ./web/dist/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -tags webapp -ldflags="-w -s" -o /octant ./cmd/octant

FROM gcr.io/distroless/static-debian13:nonroot AS final
WORKDIR /
COPY --link --from=binary-builder /octant /octant
CMD ["/octant"]