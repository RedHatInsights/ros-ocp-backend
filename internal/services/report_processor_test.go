package services

import (
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// These tests verify that ProcessReport handles poison messages (invalid JSON,
// unmarshal failures, validation failures) gracefully — returning early without
// panicking or entering an infinite loop. The nil consumer is safe because
// commitOnPermanentFailure guards against it.

func TestProcessReport_InvalidJSON_ReturnsEarly(t *testing.T) {
	msg := &kafka.Message{
		Value:          []byte("{not valid json!!!"),
		TopicPartition: kafka.TopicPartition{Partition: 0},
	}
	// nil consumer: commitOnPermanentFailure checks for nil before committing
	ProcessReport(msg, nil)
	// If we reach here without panic, the poison message path works.
}

func TestProcessReport_UnmarshalError_ReturnsEarly(t *testing.T) {
	// Valid JSON but doesn't match KafkaMsg struct fields
	msg := &kafka.Message{
		Value:          []byte(`{"unexpected_field": 42}`),
		TopicPartition: kafka.TopicPartition{Partition: 0},
	}
	ProcessReport(msg, nil)
}

func TestProcessReport_ValidationError_ReturnsEarly(t *testing.T) {
	// Valid JSON, unmarshals to KafkaMsg, but fails validation (missing required fields)
	msg := &kafka.Message{
		Value:          []byte(`{"request_id":"","b64_identity":"","metadata":{},"files":[]}`),
		TopicPartition: kafka.TopicPartition{Partition: 0},
	}
	ProcessReport(msg, nil)
}
