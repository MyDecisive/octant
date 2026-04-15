# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25

FROM node:lts-alpine as ui-builder
WORKDIR /web
COPY --from=ui-repo . .
RUN npm install && npm run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS binary-builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src

COPY --link go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY --link . .
COPY --from=ui-builder /web/dist ./web
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -tags webapp -ldflags="-w -s" -o /octant ./cmd/octant

FROM gcr.io/distroless/static-debian13:nonroot AS final
WORKDIR /
COPY --link --from=binary-builder /octant /octant
CMD ["/octant"]