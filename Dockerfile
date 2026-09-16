# Build the manager binary
# Nyamber depends on cybozu-go/placemat. Since cybozu-go/placemat does not support Ubuntu 24.04 yet, use a jammy-based image.
FROM ghcr.io/cybozu/golang:1.27.1.1_jammy@sha256:cf6a363d2a421e18a4e0038b69f0b44061903a98ab4c58ea46a68c271e81b327 AS builder

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
FROM ghcr.io/cybozu/ubuntu:22.04.20260916@sha256:fb80e128ca6b7fd2a3e165e18df5defb2eb677f207d45b7d6dc35ed1597a4cd4
LABEL org.opencontainers.image.source=https://github.com/cybozu-go/nyamber

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
