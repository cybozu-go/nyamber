# Build the manager binary
# Nyamber depends on cybozu-go/placemat. Since cybozu-go/placemat does not support Ubuntu 24.04 yet, use a jammy-based image.
FROM ghcr.io/cybozu/golang:1.26.4.1_jammy@sha256:4273ab54d46bc2018b65785354589f6af6995c42b71c290b702035f2defa088d AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY hooks/ hooks/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/nyamber-controller/main.go

# Nyamber depends on cybozu-go/placemat. Since cybozu-go/placemat does not support Ubuntu 24.04 yet, use a jammy-based image.
FROM ghcr.io/cybozu/ubuntu:22.04.20260605@sha256:2ec1363fa00398af0f13a5baa6c84d3245615de7ad576c498fe3c739fd06076e
LABEL org.opencontainers.image.source=https://github.com/cybozu-go/nyamber

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
