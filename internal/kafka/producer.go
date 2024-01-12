package kafka

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/sirupsen/logrus"
)

var p *kafka.Producer = nil
var log *logrus.Entry = nil

func startProducer() {
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

func SendMessage(msg []byte, topic string, key string) error {
	if p == nil {
		log = logging.GetLogger()
		log.Info("initializing kafka producer")
		startProducer()
	}
	delivery_chan := make(chan kafka.Event)
	defer close(delivery_chan)
	err := p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          []byte(msg),
	}, delivery_chan)
	if err != nil {
		log.Errorf("Failed to produce message to kafka: %v\n", err)
		return err
	}
	e := <-delivery_chan
	m := e.(*kafka.Message)
	if m.TopicPartition.Error != nil {
		log.Errorf("Delivery failed: %v\n", m.TopicPartition.Error)
		return m.TopicPartition.Error
	} else {
		log.Debugf("Delivered message to topic %s [%d] at offset %v\n",
			*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
		return nil
	}

}
