include scripts/.env

identity={"identity": {"org_id": "3340851", "type": "System", "auth_type": "cert-auth", "system": {"cn": "1b36b20f-7fa0-4454-a6d2-008294e06378", "cert_type": "system"}, "internal": {"org_id": "3340851", "auth_time": 6300}}}
b64_identity=$(shell echo '${identity}' | base64 -w 0 -)
ros_ocp_msg='{"request_id": "uuid1234", "b64_identity": "test", "metadata": {"account": "123", "org_id": "345", "source_id": "111", "cluster_id": "222"}, "files": ["http://dhcp131-80.gsslab.pnq2.redhat.com/rosocp/ros-usage.csv"]}'

file=./scripts/samples/cost-mgmt.tar.gz
INGRESS_PORT ?= 3000


LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@echo "🤖 Ensuring $(LOCALBIN) is available"
	mkdir -p $(LOCALBIN)
	@echo "✅ Done"


.PHONY: golangci-lint
GOLANGCILINT := $(LOCALBIN)/golangci-lint
GOLANGCI_URL := https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh
golangci-lint: $(LOCALBIN)
ifeq (,$(wildcard $(GOLANGCILINT)))
	@ echo "📥 Downloading golangci-lint"
	curl -sSfL $(GOLANGCI_URL) | sh -s -- -b $(LOCALBIN) $(GOLANGCI_VERSION)
	@ echo "✅ Done"
endif

.PHONY: db-migrate
db-migrate:
	go run rosocp.go db migrate

.PHONY: run-processor
run-processor:
	go run rosocp.go start processor

.PHONY: build
build:
	go build -o bin/rosocp rosocp.go

.PHONY: lint
lint: golangci-lint
	$(GOLANGCILINT) run ./...

.PHONY: test
test:
	go test -v ./...

local-upload-data:
	curl -vvvv -F "upload=@$(file);type=application/application/vnd.redhat.hccm.tar+tgz" \
		-H "x-rh-identity: ${b64_identity}" \
		-H "x-rh-request_id: testtesttest" \
		http://localhost:${INGRESS_PORT}/api/ingress/v1/upload

upload-msg-to-rosocp:
	echo ${ros_ocp_msg} | docker-compose -f scripts/docker-compose.yml exec -T kafka kafka-console-producer --topic platform.upload.rosocp  --broker-list localhost:29092
