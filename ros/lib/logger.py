import logging
import os
import sys
import socket
from threading import local
from ros.lib.config import LOG_LEVEL
from logstash_formatter import LogstashFormatterV1


threadctx = local()


def cloudwatch_config_values():
    # Returns Cloudwatch config values
    if os.environ.get("ACG_CONFIG"):
        import app_common_python

        cfg = app_common_python.LoadedConfig
        if cfg.logging:
            cw = cfg.logging.cloudwatch
            return cw.accessKeyId, cw.secretAccessKey, cw.region, cw.logGroup
        else:
            return None, None, None, None


def initialize_logging():
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(LogstashFormatterV1())
    handler.addFilter(ContextualFilter())
    logging.root.setLevel(LOG_LEVEL)
    logging.root.addHandler(handler)
    if os.environ.get("CLOWDER_ENABLED", "").lower() == "true":
        aws_access_key_id, aws_secret_access_key, aws_region_name, aws_log_group = cloudwatch_config_values()
        if all((aws_access_key_id, aws_secret_access_key, aws_region_name, aws_log_group)):
            import boto3
            import watchtower

            boto3_client = boto3.client(
                'logs',
                aws_access_key_id=aws_access_key_id,
                aws_secret_access_key=aws_secret_access_key,
                region_name=aws_region_name
            )

            # configure logging to use watchtower
            cw_handler = watchtower.CloudWatchLogHandler(
                boto3_client=boto3_client,
                log_group=aws_log_group,
                stream_name=socket.gethostname()
            )

            cw_handler.setFormatter(LogstashFormatterV1())
            cw_handler.addFilter(ContextualFilter())
            logging.root.addHandler(cw_handler)


class ContextualFilter(logging.Filter):
    """
    This filter gets the request_id and tenant info from the message and adds to
    each log record. This way, every time it won't require to retrieve these details
    per log message.
    """

    def filter(self, log_record):
        try:
            log_record.request_id = threadctx.request_id
        except Exception:
            log_record.request_id = "-1"

        try:
            log_record.account = threadctx.account
        except Exception:
            log_record.account = "000001"

        try:
            log_record.org_id = threadctx.org_id
        except Exception:
            log_record.org_id = "000001"

        return True
