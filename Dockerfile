FROM registry.redhat.io/ubi8/go-toolset:latest as builder
WORKDIR /go/src/app
COPY . .
USER 0
RUN go get -d ./... && \
    go build -o rosocp cmd/rosocp-consumer/main.go

FROM registry.redhat.io/ubi8/ubi-minimal:latest
WORKDIR /
COPY --from=builder /go/src/app/rosocp ./rosocp
USER 1001
CMD ["/rosocp"]
