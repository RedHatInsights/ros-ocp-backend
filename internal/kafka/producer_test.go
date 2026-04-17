package kafka

import (
	"strings"
	"testing"
)

func TestSendMessage_NilProducer_ReturnsError(t *testing.T) {
	origP := p
	origInit := initProducer
	defer func() {
		p = origP
		initProducer = origInit
	}()

	p = nil
	initProducer = func() {} // no-op: simulates a failed initialization

	err := SendMessage([]byte("test"), "test-topic", "test-key")
	if err == nil {
		t.Fatal("expected error when producer is nil, got nil")
	}
	if !strings.Contains(err.Error(), "producer failed to initialize") {
		t.Errorf("unexpected error message: %v", err)
	}
}
