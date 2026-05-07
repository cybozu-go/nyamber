# Build the manager binary
FROM ghcr.io/cybozu/golang:1.26.2.2_jammy@sha256:ce7b7016908ee9bebefd48aa18106cadf3e7d7ac6150b8e789726cda3cc9beae AS builder

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

FROM ghcr.io/cybozu/ubuntu:22.04.20260422@sha256:465d53037c713b03e2d84bdec49903513b50da16eebfd06d3ca9aaa12bff4407
LABEL org.opencontainers.image.source=https://github.com/cybozu-go/nyamber

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
