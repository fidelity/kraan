# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Copy the go source
COPY pkg/ pkg
COPY main/ main
COPY controllers/ controllers
COPY api/ api
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

ARG TARGETARCH
ENV GOARCH=${TARGETARCH}

# Build
RUN mkdir bin
RUN CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" GO111MODULE=on go build -a -o bin/kraan-controller main/main.go
RUN apt install -y curl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.21.0/bin/linux/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl
RUN mv kubectl bin
RUN curl -LO https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.8.6/kustomize_v3.8.6_linux_${TARGETARCH}.tar.gz
RUN tar xzf ./kustomize_v3.8.6_linux_${TARGETARCH}.tar.gz
RUN chmod +x ./kustomize
RUN mv kustomize bin
RUN rm ./kustomize_v3.8.6_linux_${TARGETARCH}.tar.gz

FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /workspace/bin/ /usr/local/bin/
USER nobody
ENTRYPOINT ["/usr/local/bin/kraan-controller"]
