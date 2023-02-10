package config

import (
	"fmt"
	"os"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type config struct {
	//Application config
	LogLevel string `mapstructure:"LogLevel"`

	//Kafka configs
	InsightsKafkaAddress string `mapstructure:"INSIGHTS_KAFKA_ADDRESS"`
	KafkaConsumerGroupId string `mapstructure:"KAFKA_CONSUMER_GROUP_ID"`
	KafkaAutoCommit      bool   `mapstructure:"KAFKA_AUTO_COMMIT"`
	UploadTopic          string `mapstructure:"UPLOAD_TOPIC"`
}

var cfg config

func InitConfig() {
	viper.AutomaticEnv()
	viper.SetDefault("INSIGHTS_KAFKA_ADDRESS", "localhost:29092")
	viper.SetDefault("UPLOAD_TOPIC", "platform.upload.rosocp")
	viper.SetDefault("KAFKA_CONSUMER_GROUP_ID", "ros-ocp")
	viper.SetDefault("KAFKA_AUTO_COMMIT", false)
	viper.SetDefault("LogLevel", "INFO")

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
		fmt.Println("Can not unmarshal config. Exiting..")
		os.Exit(1)
	}
}

func Get() *config {
	return &cfg
}
