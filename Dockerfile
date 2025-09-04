# Build the manager binary
FROM golang:1.22.6 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

ARG package=.
ARG ARCH
ARG LDFLAGS

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY cloud/ cloud/
COPY exp/ exp/
COPY feature/ feature/
COPY version/ version/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "${LDFLAGS} -extldflags '-static'"  -o manager ${package}

FROM ghcr.io/oracle/oraclelinux:9-slim
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
