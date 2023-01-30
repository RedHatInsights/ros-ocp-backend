import logging
import json

from ros_ocp.lib.consume import init_consumer
from ros_ocp.lib.logger import initialize_logging, threadctx
from ros_ocp.lib.config import KAFKA_BROKER, UPLOAD_TOPIC, KAFKA_AUTO_COMMIT, KAFKA_CA_FILE_PATH
from ros_ocp.lib.utils import generate_request_object
from ros_ocp.lib.exceptions import KafkaMsgException
from ros_ocp.processor.report_processor import process_report


initialize_logging()
LOG = logging.getLogger(__name__)

# Create cacert for kafka managed kafka config.
if KAFKA_BROKER and KAFKA_BROKER.cacert:
    with open(KAFKA_CA_FILE_PATH, 'w') as f:
        f.write(KAFKA_BROKER.cacert)


def set_extra_log_data(request_obj):
    threadctx.request_id = request_obj['request_id']
    threadctx.account = request_obj['account']
    threadctx.org_id = request_obj['org_id']


consumer = init_consumer()

LOG.info(f"Started listening on kafka topic - {UPLOAD_TOPIC}.")

while True:
    msg = consumer.poll(1.0)
    if msg is None:
        continue
    if msg.error():
        LOG.error(f"Kafka error occured : {msg.error()}.")
        continue
    try:
        msg = json.loads(msg.value().decode("utf-8"))
        request_obj = generate_request_object(msg)
        set_extra_log_data(request_obj)
        consumer.commit()
        process_report(msg, request_obj)
    except KafkaMsgException as err:
        LOG.error(f"Incorrect event received on kafka topic: {err}")
    except Exception as error:
        LOG.error(f"[listen_for_messages] UNKNOWN error encountered: {type(error).__name__}: {error}", exc_info=True)
    finally:
        if not KAFKA_AUTO_COMMIT:
            consumer.commit()
