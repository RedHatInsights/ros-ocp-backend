import logging

from ros.lib.config import UPLOAD_TOPIC
from ros.lib.exceptions import KafkaMsgException


LOG = logging.getLogger(__name__)


def generate_request_object(consumed_message):
    metadata = consumed_message.get('metadata')
    if not metadata:
        raise KafkaMsgException("Message missing metadata field.")
    org_id = metadata.get('org_id')
    LOG.info(f"Received record on {UPLOAD_TOPIC} topic for org_id {org_id}.")
    missing_fields = []
    request_id = consumed_message.get('request_id')
    cluster_id = metadata.get('cluster_id')
    if not org_id:
        missing_fields.append('org_id')
    if not request_id:
        missing_fields.append('request_id')
    if not cluster_id:
        missing_fields.append('cluster_id')
    if missing_fields:
        raise KafkaMsgException(f"Message missing required field(s): {', '.join(missing_fields)}.")
    request_obj = {
        'request_id': request_id,
        'account': metadata.get('account'),
        'org_id': org_id,
        'b64_identity': consumed_message.get('b64_identity')
    }
    return request_obj
