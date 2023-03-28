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
	ExperimentsTopic      string `mapstructure:"EXPERIMENTS_TOPIC"`
	KafkaUsername         string
	KafkaPassword         string
	KafkaSASLMechanism    string
	KafkaSecurityProtocol string
	KafkaCA               string

	// Kruize config
	KruizeUrl      string `mapstructure:"KRUIZE_URL"`
	KruizeWaitTime string `mapstructure:"KRUIZE_WAIT_TIME"`

	// Database config
	DBName     string `mapstructure:"ROSOCP_DB_NAME"`
	DBUser     string `mapstructure:"ROSOCP_DB_USER"`
	DBPassword string `mapstructure:"ROSOCP_DB_PASSWORD"`
	DBHost     string `mapstructure:"ROSOCP_DB_HOST"`
	DBPort     string `mapstructure:"ROSOCP_DB_PORT"`
	DBssl      string `mapstructure:"ROSOCP_DB_SSL"`

	API_PORT string
}

var cfg *Config = nil

func initConfig() {
	viper.AutomaticEnv()
	if clowder.IsClowderEnabled() {
		c := clowder.LoadedConfig
		broker := c.Kafka.Brokers[0]
		viper.SetDefault("KAFKA_BOOTSTRAP_SERVERS", strings.Join(clowder.KafkaServers, ","))
		viper.SetDefault("UPLOAD_TOPIC", clowder.KafkaTopics["hccm.ros.events"].Name)
		viper.SetDefault("EXPERIMENTS_TOPIC", clowder.KafkaTopics["rosocp.kruize.experiments"].Name)

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
		viper.SetDefault("ROSOCP_DB_NAME", c.Database.Name)
		viper.SetDefault("ROSOCP_DB_USER", c.Database.Username)
		viper.SetDefault("ROSOCP_DB_PASSWORD", c.Database.Password)
		viper.SetDefault("ROSOCP_DB_HOST", c.Database.Hostname)
		viper.SetDefault("ROSOCP_DB_PORT", c.Database.Port)
		viper.SetDefault("ROSOCP_DB_SSL", c.Database.SslMode)

	} else {
		viper.SetDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:29092")
		viper.SetDefault("UPLOAD_TOPIC", "hccm.ros.events")
		viper.SetDefault("EXPERIMENTS_TOPIC", "rosocp.kruize.experiments")

		// default DB Config
		viper.SetDefault("ROSOCP_DB_NAME", "postgres")
		viper.SetDefault("ROSOCP_DB_USER", "postgres")
		viper.SetDefault("ROSOCP_DB_PASSWORD", "postgres")
		viper.SetDefault("ROSOCP_DB_HOST", "localhost")
		viper.SetDefault("ROSOCP_DB_PORT", "15432")
		viper.SetDefault("ROSOCP_DB_SSL", "disable")

		//default RBAC Config
		viper.SetDefault("RBACHost", "localhost")
		viper.SetDefault("RBACPort", "9080")
		viper.SetDefault("RBACProtocol", "http")
		viper.SetDefault("RBACEnabled", false)

	}
	viper.SetDefault("API_PORT", "8000")
	viper.SetDefault("KRUIZE_WAIT_TIME", "30")
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
		fmt.Println("Config initialized")
	}
	return cfg
}
