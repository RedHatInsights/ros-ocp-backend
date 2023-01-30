import os
import logging

LOG = logging.getLogger(__name__)

CLOWDER_ENABLED = True if os.getenv("CLOWDER_ENABLED", default="False").lower() in ["true", "t", "yes", "y"] else False


def kafka_auth_config(connection_object):
    if KAFKA_BROKER:
        if KAFKA_BROKER.cacert:
            connection_object["ssl.ca.location"] = KAFKA_CA_FILE_PATH
        if KAFKA_BROKER.sasl and KAFKA_BROKER.sasl.username:
            connection_object.update({
                "security.protocol": KAFKA_BROKER.sasl.securityProtocol,
                "sasl.mechanisms": KAFKA_BROKER.sasl.saslMechanism,
                "sasl.username": KAFKA_BROKER.sasl.username,
                "sasl.password": KAFKA_BROKER.sasl.password,
            })
    return connection_object


if CLOWDER_ENABLED:
    LOG.info("Using Clowder Operator...")
    from app_common_python import LoadedConfig, KafkaTopics
    KAFKA_BROKER = LoadedConfig.kafka.brokers[0]
    INSIGHTS_KAFKA_ADDRESS = KAFKA_BROKER.hostname + ":" + str(KAFKA_BROKER.port)
    UPLOAD_TOPIC = KafkaTopics["platform.upload.rosocp"].name
    METRICS_PORT = LoadedConfig.metricsPort
else:
    INSIGHTS_KAFKA_HOST = os.getenv('INSIGHTS_KAFKA_HOST', 'localhost')
    INSIGHTS_KAFKA_PORT = os.getenv('INSIGHTS_KAFKA_PORT', '29092')
    INSIGHTS_KAFKA_ADDRESS = f'{INSIGHTS_KAFKA_HOST}:{INSIGHTS_KAFKA_PORT}'
    KAFKA_BROKER = None
    UPLOAD_TOPIC = os.getenv('UPLOAD_TOPIC', 'platform.upload.rosocp')
    METRICS_PORT = os.getenv("METRICS_PORT", 5005)

KRUIZE_HOST = os.getenv("KRUIZE_HOST", "localhost")
KRUIZE_PORT = os.getenv("KRUIZE_PORT", 8080)
KRUIZE_URL = f'http://{KRUIZE_HOST}:{KRUIZE_PORT}'
LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO").upper()
KAFKA_AUTO_COMMIT = os.getenv("KAFKA_AUTO_COMMIT", False)
KAFKA_CONSUMER_GROUP_ID = os.getenv("KAFKA_CONSUMER_GROUP_ID", "ros-ocp")
KAFKA_CA_FILE_PATH = "/tmp/cacert"
