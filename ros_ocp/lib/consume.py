from confluent_kafka import Consumer
from ros_ocp.lib.config import (
        KAFKA_AUTO_COMMIT,
        INSIGHTS_KAFKA_ADDRESS,
        UPLOAD_TOPIC,
        KAFKA_CONSUMER_GROUP_ID,
        kafka_auth_config
    )


def init_consumer():
    connection_object = {
        'bootstrap.servers': INSIGHTS_KAFKA_ADDRESS,
        'group.id': KAFKA_CONSUMER_GROUP_ID,
        'enable.auto.commit': KAFKA_AUTO_COMMIT
    }
    kafka_auth_config(connection_object)
    consumer = Consumer(connection_object)
    consumer.subscribe([UPLOAD_TOPIC])

    return consumer
