FROM registry.access.redhat.com/ubi9/go-toolset:1.26.5 AS builder
WORKDIR /go/src/app
COPY . .
USER 0
# Source prefetch env to use hermetic cache for Go modules
RUN if [ -f /cachi2/cachi2.env ]; then . /cachi2/cachi2.env; fi && \
    go get -d ./... && \
    go build -o rosocp rosocp.go && \
    echo "$(go version)" > go_version_details

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /
COPY --from=builder /go/src/app/rosocp ./rosocp
COPY --from=builder /go/src/app/go_version_details ./go_version_details
COPY migrations ./migrations
COPY openapi.json ./openapi.json
COPY resource_optimization_openshift.json ./resource_optimization_openshift.json

ARG IMAGE_NAME
ARG VERSION
LABEL summary="Resource Optimization for OpenShift backend for on-premise deployments" \
    description="Red Hat Resource Optimization for OpenShift backend for on-premise deployments" \
    io.k8s.description="Red Hat Resource Optimization for OpenShift backend for on-premise deployments" \
    io.k8s.display-name="Resource Optimization for OpenShift Backend" \
    com.redhat.component="costmanagement-ros-ocp-backend-rhel9-container" \
    name="$IMAGE_NAME" \
    version="$VERSION" \
    release="1" \
    vendor="Red Hat, Inc." \
    distribution-scope="public" \
    cpe="cpe:/a:redhat:cost_management_on_premise:1::el9" \
    maintainer="Red Hat Cost Management Services <cost-mgmt@redhat.com>"

USER 1001
