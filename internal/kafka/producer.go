package kafka

import (
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/sirupsen/logrus"
)

var p *kafka.Producer = nil
var log *logrus.Entry = nil

var initProducer = defaultStartProducer

func startProducer() { initProducer() }

func defaultStartProducer() {
	cfg := config.GetConfig()
	var configMap kafka.ConfigMap

	if cfg.KafkaSASLMechanism != "" {
		configMap = kafka.ConfigMap{
			"bootstrap.servers":        cfg.KafkaBootstrapServers,
			"go.delivery.reports":      true,
			"security.protocol":        cfg.KafkaSecurityProtocol,
			"sasl.mechanism":           cfg.KafkaSASLMechanism,
			"ssl.ca.location":          cfg.KafkaCA,
			"sasl.username":            cfg.KafkaUsername,
			"sasl.password":            cfg.KafkaPassword,
			"allow.auto.create.topics": true,
		}

	} else {
		configMap = kafka.ConfigMap{
			"bootstrap.servers":        cfg.KafkaBootstrapServers,
			"go.delivery.reports":      true,
			"enable.auto.commit":       cfg.KafkaAutoCommit,
			"go.logs.channel.enable":   true,
			"allow.auto.create.topics": true,
		}
	}

	producer, err := kafka.NewProducer(&configMap)
	if err != nil {
		log.Errorf("Error creating kafka producer")
		return
	}

	p = producer

}

const sendMessageMaxRetries = 3

func SendMessage(msg []byte, topic string, key string) error {
	if log == nil {
		log = logging.GetLogger()
	}
	if p == nil {
		log.Info("initializing kafka producer")
		startProducer()
	}
	if p == nil {
		return fmt.Errorf("kafka producer failed to initialize; cannot send message to topic %s", topic)
	}

	var lastErr error
	for attempt := 0; attempt < sendMessageMaxRetries; attempt++ {
		lastErr = sendMessageOnce(msg, topic, key)
		if lastErr == nil {
			return nil
		}
		if attempt < sendMessageMaxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			log.Warnf("SendMessage attempt %d/%d failed (topic=%s, key=%s): %v; retrying in %v",
				attempt+1, sendMessageMaxRetries, topic, key, lastErr, backoff)
			time.Sleep(backoff)
		}
	}
	log.Errorf("SendMessage exhausted %d retries (topic=%s, key=%s): %v", sendMessageMaxRetries, topic, key, lastErr)
	return lastErr
}

func sendMessageOnce(msg []byte, topic string, key string) error {
	delivery_chan := make(chan kafka.Event)
	defer close(delivery_chan)
	err := p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          []byte(msg),
	}, delivery_chan)
	if err != nil {
		return fmt.Errorf("produce failed: %w", err)
	}
	e := <-delivery_chan
	m, ok := e.(*kafka.Message)
	if !ok {
		return fmt.Errorf("unexpected delivery event type: %T", e)
	}
	if m.TopicPartition.Error != nil {
		return fmt.Errorf("delivery failed: %w", m.TopicPartition.Error)
	}
	log.Debugf("Delivered message to topic %s [%d] at offset %v",
		*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
	return nil
}
