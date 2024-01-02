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

func StartConsumer(kafka_topic string, handler func(msg *kafka.Message, consumer_object *kafka.Consumer), auto_commit_option ...bool) {
	log := logging.GetLogger()
	cfg := config.GetConfig()

	// Fetch and validate auto_commit_option value
	var auto_commit bool
	if len(auto_commit_option) > 0 && !auto_commit_option[0] {
		auto_commit = auto_commit_option[0]
	} else {
		auto_commit = cfg.KafkaAutoCommit
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	var configMap kafka.ConfigMap
	if cfg.KafkaSASLMechanism != "" {
		configMap = kafka.ConfigMap{
			"bootstrap.servers":        cfg.KafkaBootstrapServers,
			"group.id":                 cfg.KafkaConsumerGroupId,
			"security.protocol":        cfg.KafkaSecurityProtocol,
			"sasl.mechanism":           cfg.KafkaSASLMechanism,
			"sasl.username":            cfg.KafkaUsername,
			"sasl.password":            cfg.KafkaPassword,
			"enable.auto.commit":       auto_commit,
			"go.logs.channel.enable":   true,
			"allow.auto.create.topics": true,
		}

		// As per librdkafka doc - https://github.com/confluentinc/librdkafka/blob/master/CONFIGURATION.md?plain=1#L73
		// Default ca location is set to ca-certificates package. i.e. /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
		if cfg.KafkaCA != "" {
			configMap["ssl.ca.location"] = cfg.KafkaCA
		}

	} else {
		configMap = kafka.ConfigMap{
			"bootstrap.servers":        cfg.KafkaBootstrapServers,
			"group.id":                 cfg.KafkaConsumerGroupId,
			"enable.auto.commit":       auto_commit,
			"go.logs.channel.enable":   true,
			"allow.auto.create.topics": true,
		}
	}

	consumer, err := kafka.NewConsumer(&configMap)
	if err != nil {
		log.Errorf("Failed to create consumer: %s", err)
		os.Exit(1)
	}

	err = consumer.Subscribe(kafka_topic, nil)
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
				log.Infof("Message received from kafka %s: %s", msg.TopicPartition, string(msg.Value))
				handler(msg, consumer)
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
