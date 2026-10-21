# Build the manager binary
# Nyamber depends on cybozu-go/placemat. Since cybozu-go/placemat does not support Ubuntu 24.04 yet, use a jammy-based image.
FROM ghcr.io/cybozu/golang:1.27.3.2_jammy@sha256:d8a293d6f77ddf961a985592d1c349b95e11029be083d9fd1d44599598219c9d AS builder

WORKDIR /workspace

ENV GOPROXY=https://golang.flatt.tech
ENV NETRC=/run/secrets/netrc

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=secret,id=netrc,target=/run/secrets/netrc,required=false \
    go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY hooks/ hooks/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=off go build -a -o manager cmd/nyamber-controller/main.go

# Nyamber depends on cybozu-go/placemat. Since cybozu-go/placemat does not support Ubuntu 24.04 yet, use a jammy-based image.
FROM ghcr.io/cybozu/ubuntu:22.04.20261021@sha256:f52eedc9de1a2b34e0ef71e4a6e1489cf3fe97e0dd09f2ceab43ddf036f6b5a3
LABEL org.opencontainers.image.source=https://github.com/cybozu-go/nyamber

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
