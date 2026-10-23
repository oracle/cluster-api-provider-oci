# Build the manager binary
ARG BUILDER_IMAGE=golang:1.26.10
FROM ${BUILDER_IMAGE} AS builder

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
RUN microdnf update -y ca-certificates && microdnf clean all && update-ca-trust
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
