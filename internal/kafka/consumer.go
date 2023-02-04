package kafka

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

func StartConsumer() {
	log := logging.GetLogger()
	cfg := config.Get()
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  cfg.InsightsKafkaAddress,
		"group.id":           cfg.KafkaConsumerGroupId,
		"enable.auto.commit": cfg.KafkaAutoCommit,
	})
	if err != nil {
		log.Errorf("Failed to create consumer: %s", err)
		os.Exit(1)
	}

	err = consumer.Subscribe(cfg.UploadTopic, nil)
	if err != nil {
		log.Errorf("Failed to create subscribe: %s", err)
	}

	run := true
	for run {
		select {
		case sig := <-sigchan:
			log.Infof("Caught Signal %v: terminating", sig)
			consumer.Close()
			os.Exit(1)
		default:
			msg, err := consumer.ReadMessage(time.Second)
			if err == nil {
				// Invoke report processor function in this block.
				log.Infof("Message on %s: %s", msg.TopicPartition, string(msg.Value))
			} else if !err.(kafka.Error).IsTimeout() {
				// The client will automatically try to recover from all errors.
				// Timeout is not considered an error because it is raised by
				// ReadMessage in absence of messages.
				log.Errorf("Consumer error: %v (%v)", err, msg)
			}
		}
	}
	consumer.Close()
}
