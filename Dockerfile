# Build the manager binary
FROM golang:1.14 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY pkg/ pkg
COPY main/ main
COPY controllers/ controllers
COPY api/ api

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o kraan-controller main/main.go

FROM alpine:3.12
WORKDIR /
RUN apk add curl
COPY --from=builder /workspace/kraan-controller /usr/local/bin/
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.17.0/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin
RUN addgroup -S controller && adduser -S -g controller controller

USER controller

ENTRYPOINT ["/usr/local/bin/kraan-controller"]
