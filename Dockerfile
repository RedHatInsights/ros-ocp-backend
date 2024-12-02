FROM registry.access.redhat.com/ubi8/go-toolset:1.22 as builder
WORKDIR /go/src/app
COPY . .
USER 0
RUN go get -d ./... && \
    go build -o rosocp rosocp.go && \
    echo "$(go version)" > go_version_details

FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /go/src/app/rosocp ./rosocp
COPY --from=builder /go/src/app/go_version_details ./go_version_details
COPY migrations ./migrations
COPY openapi.json ./openapi.json
COPY resource_optimization_openshift.json ./resource_optimization_openshift.json
