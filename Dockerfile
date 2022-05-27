# Build the manager binary
FROM golang:1.17 as builder

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
ARG TARGETOS

# Build
RUN mkdir bin
RUN apt install -y curl tar
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.21.0/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl
RUN mv kubectl bin
RUN curl -LO https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.8.7/kustomize_v3.8.7_${TARGETOS}_${TARGETARCH}.tar.gz
RUN tar xzvf ./kustomize_v3.8.7_${TARGETOS}_${TARGETARCH}.tar.gz
RUN chmod +x ./kustomize
RUN mv kustomize bin
RUN rm ./kustomize_v3.8.7_${TARGETOS}_${TARGETARCH}.tar.gz
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH="${TARGETARCH}" GO111MODULE=on go build -a -o bin/kraan-controller main/main.go

FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /workspace/bin/ /usr/local/bin/
USER 1000
ENTRYPOINT ["/usr/local/bin/kraan-controller"]
