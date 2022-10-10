identity={"identity": {"org_id": "3340851", "type": "System", "auth_type": "cert-auth", "system": {"cn": "1b36b20f-7fa0-4454-a6d2-008294e06378", "cert_type": "system"}, "internal": {"org_id": "3340851", "auth_time": 6300}}}
b64_identity=$(shell echo '${identity}' | base64 -w 0 -)

file=./scripts/samples/cost-mgmt.tar.gz

local-upload-data:
	curl -vvvv -F "upload=@$(file);type=application/application/vnd.redhat.hccm.tar+tgz" \
		-H "x-rh-identity: ${b64_identity}" \
		-H "x-rh-request_id: testtesttest" \
		http://localhost:3000/api/ingress/v1/upload
