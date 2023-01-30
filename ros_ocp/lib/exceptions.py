class KafkaMsgException(Exception):
    """Use to report errors with kafka message.
    Used when we think the kafka message is useful
    in debugging.  Error with external services
    (connected via kafka).
    """

    pass


class FailDownloadException(Exception):
    """Use to report download errors that should not be retried."""

    pass
