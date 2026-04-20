package housekeeper

import (
	"encoding/json"
	"testing"

	k "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupBrokenDB(t *testing.T) func() {
	t.Helper()
	origDB := database.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	database.DB = db
	return func() { database.DB = origDB }
}

func makeSourcesDestroyMessage(t *testing.T, event types.SourcesEvent) *k.Message {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return &k.Message{
		Value: body,
		Headers: []k.Header{
			{Key: "event_type", Value: []byte("Application.destroy")},
		},
		TopicPartition: k.TopicPartition{Partition: 0},
	}
}

func TestSourcesListener_DBLookupError_ReturnsEarly(t *testing.T) {
	restore := setupBrokenDB(t)
	defer restore()

	// Set cost_app_id to match the event so the DB lookup path is triggered
	cost_app_id = 99

	msg := makeSourcesDestroyMessage(t, types.SourcesEvent{
		Id:                  1,
		Source_id:           42,
		Application_type_id: 99,
		Tenant:              "test-tenant",
	})

	// sourcesListener should handle the DB error gracefully (log + return),
	// not panic or proceed with a zero-value Cluster.
	sourcesListener(msg, nil)
}

func TestSourcesListener_InvalidJSON_ReturnsEarly(t *testing.T) {
	restore := setupBrokenDB(t)
	defer restore()

	msg := &k.Message{
		Value: []byte("{not json"),
		Headers: []k.Header{
			{Key: "event_type", Value: []byte("Application.destroy")},
		},
		TopicPartition: k.TopicPartition{Partition: 0},
	}

	sourcesListener(msg, nil)
}

func TestSourcesListener_NonMatchingEventType_NoOp(t *testing.T) {
	restore := setupBrokenDB(t)
	defer restore()

	msg := &k.Message{
		Value: []byte(`{}`),
		Headers: []k.Header{
			{Key: "event_type", Value: []byte("Source.create")},
		},
		TopicPartition: k.TopicPartition{Partition: 0},
	}

	sourcesListener(msg, nil)
}
