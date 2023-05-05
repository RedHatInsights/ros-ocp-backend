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
	ServiceName string `mapstructure:"ServiceName"`
	LogLevel    string `mapstructure:"LogLevel"`
	LogFormater string `mapstructure:"LogFormater"`

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
	DBName     string
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBssl      string
	DBCACert   string

	//RBAC config
	RBACHost     string
	RBACPort     string
	RBACProtocol string
	RBACEnabled  bool

	API_PORT string

	// Cloudwatch config
	CwLogGroup  string
	CwRegion    string
	CwAccessKey string
	CwSecretKey string
}

var cfg *Config = nil

func initConfig() {
	viper.AutomaticEnv()
	if clowder.IsClowderEnabled() {
		viper.SetDefault("LogFormater", "json")

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
			viper.Set("KafkaSecurityProtocol", broker.Sasl.SecurityProtocol) //nolint:all
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
		viper.SetDefault("DBssl", c.Database.SslMode)
		viper.SetDefault("DBCACert", c.Database.RdsCa)

		// clowder RBAC Config
		for _, endpoint := range c.Endpoints {
			if endpoint.App == "rbac" {
				viper.SetDefault("RBACHost", endpoint.Hostname)
				viper.SetDefault("RBACPort", endpoint.Port)
				viper.SetDefault("RBACProtocol", "http")
				viper.SetDefault("RBACEnabled", true)
			}
		}

		//clowder cloudwatch config
		viper.SetDefault("CwLogGroup", c.Logging.Cloudwatch.LogGroup)
		viper.SetDefault("CwRegion", c.Logging.Cloudwatch.Region)
		viper.SetDefault("CwAccessKey", c.Logging.Cloudwatch.AccessKeyId)
		viper.SetDefault("CwSecretKey", c.Logging.Cloudwatch.SecretAccessKey)

	} else {
		viper.SetDefault("LogFormater", "text")

		viper.SetDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:29092")
		viper.SetDefault("UPLOAD_TOPIC", "hccm.ros.events")
		viper.SetDefault("EXPERIMENTS_TOPIC", "rosocp.kruize.experiments")

		// default DB Config
		viper.SetDefault("DBName", "postgres")
		viper.SetDefault("DBUser", "postgres")
		viper.SetDefault("DBPassword", "postgres")
		viper.SetDefault("DBHost", "localhost")
		viper.SetDefault("DBPort", "15432")
		viper.SetDefault("DBssl", "disable")
		viper.SetDefault("DBCACert", "")

		//default RBAC Config
		viper.SetDefault("RBACHost", "localhost")
		viper.SetDefault("RBACPort", "9080")
		viper.SetDefault("RBACProtocol", "http")
		viper.SetDefault("RBACEnabled", false)

	}
	viper.SetDefault("ServiceName", "rosocp")
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
