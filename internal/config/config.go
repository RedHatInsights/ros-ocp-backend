package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

type Config struct {
	//Application config
	LogLevel string `mapstructure:"LogLevel"`

	//Kafka configs
	KafkaBootstrapServers string `mapstructure:"KAFKA_BOOTSTRAP_SERVERS"`
	KafkaConsumerGroupId  string `mapstructure:"KAFKA_CONSUMER_GROUP_ID"`
	KafkaAutoCommit       bool   `mapstructure:"KAFKA_AUTO_COMMIT"`
	UploadTopic           string `mapstructure:"UPLOAD_TOPIC"`
	KafkaUsername         string
	KafkaPassword         string
	KafkaSASLMechanism    string
	KafkaSecurityProtocol string
	KafkaCA               string

	// Kruize config
	KruizeUrl string `mapstructure:"KRUIZE_URL"`

	// Database config
	DBName     string
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
}

var cfg *Config = nil

func initConfig() {
	viper.AutomaticEnv()
	if clowder.IsClowderEnabled() {
		c := clowder.LoadedConfig
		broker := c.Kafka.Brokers[0]
		viper.SetDefault("KAFKA_BOOTSTRAP_SERVERS", strings.Join(clowder.KafkaServers, ","))
		viper.SetDefault("UPLOAD_TOPIC", clowder.KafkaTopics["platform.upload.rosocp"].Name)

		// Kafka SSL Config
		if broker.Authtype != nil {
			viper.Set("KafkaUsername", broker.Sasl.Username)
			viper.Set("KafkaPassword", broker.Sasl.Password)
			viper.Set("KafkaSASLMechanism", broker.Sasl.SaslMechanism)
			viper.Set("KafkaSecurityProtocol", broker.SecurityProtocol)
		}

		if broker.Cacert != nil {
			caPath, err := c.KafkaCa(broker)
			if err != nil {
				panic("Kafka CA failed to write")
			}
			viper.Set("KafkaCA", caPath)
		}

		// clowder DB Config
		viper.SetDefault("DBName", c.Database.Name)
		viper.SetDefault("DBUser", c.Database.Username)
		viper.SetDefault("DBPassword", c.Database.Password)
		viper.SetDefault("DBHost", c.Database.Hostname)
		viper.SetDefault("DBPort", c.Database.Port)

	} else {
		viper.SetDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:29092")
		viper.SetDefault("UPLOAD_TOPIC", "platform.upload.rosocp")

		// default DB Config
		viper.SetDefault("DBName", "postgres")
		viper.SetDefault("DBUser", "postgres")
		viper.SetDefault("DBPassword", "postgres")
		viper.SetDefault("DBHost", "localhost")
		viper.SetDefault("DBPort", "15432")
	}

	viper.SetDefault("KAFKA_CONSUMER_GROUP_ID", "ros-ocp")
	viper.SetDefault("KAFKA_AUTO_COMMIT", false)
	viper.SetDefault("LogLevel", "INFO")
	viper.SetDefault("KRUIZE_HOST", "localhost")
	viper.SetDefault("KRUIZE_PORT", "8080")
	viper.SetDefault("KRUIZE_URL", fmt.Sprintf("http://%s:%s", viper.GetString("KRUIZE_HOST"), viper.GetString("KRUIZE_PORT")))

	// Hack till viper issue get fix - https://github.com/spf13/viper/issues/761
	envKeysMap := &map[string]interface{}{}
	if err := mapstructure.Decode(cfg, &envKeysMap); err != nil {
		fmt.Println(err)
	}
	for k := range *envKeysMap {
		if bindErr := viper.BindEnv(k); bindErr != nil {
			fmt.Println(bindErr)
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Println("Can not unmarshal config. Exiting.. ", err)
		os.Exit(1)
	}
}

func GetConfig() *Config {
	if cfg == nil {
		initConfig()
	}
	return cfg
}
