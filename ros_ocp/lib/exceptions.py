class KafkaMsgException(Exception):
    """Use to report errors with kafka message.
    Used when we think the kafka message is useful
    in debugging.  Error with external services
    (connected via kafka).
    """

    pass
