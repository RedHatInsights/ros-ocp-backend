package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestEnvironmentVariableConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		viperKey     string
		defaultValue string
		expected     string
		setEnv       bool
	}{
		{
			name:         "DB_HOST environment variable overrides default",
			envKey:       "DB_HOST",
			envValue:     "custom-db-host",
			viperKey:     "DBHost",
			defaultValue: "localhost",
			expected:     "custom-db-host",
			setEnv:       true,
		},
		{
			name:         "DB_PORT uses default when environment variable not set",
			envKey:       "DB_PORT",
			envValue:     "",
			viperKey:     "DBPort",
			defaultValue: "15432",
			expected:     "15432",
			setEnv:       false,
		},
		{
			name:         "KAFKA_BOOTSTRAP_SERVERS environment variable overrides default",
			envKey:       "KAFKA_BOOTSTRAP_SERVERS",
			envValue:     "kafka:9092",
			viperKey:     "KAFKA_BOOTSTRAP_SERVERS",
			defaultValue: "localhost:29092",
			expected:     "kafka:9092",
			setEnv:       true,
		},
		{
			name:         "DB_CA_CERT environment variable overrides default",
			envKey:       "DB_CA_CERT",
			envValue:     "test-ca-cert",
			viperKey:     "DBCACert",
			defaultValue: "",
			expected:     "test-ca-cert",
			setEnv:       true,
		},
		{
			name:         "SOURCES_API_BASE_URL uses default when not set",
			envKey:       "SOURCES_API_BASE_URL",
			envValue:     "",
			viperKey:     "SOURCES_API_BASE_URL",
			defaultValue: "http://127.0.0.1:8002",
			expected:     "http://127.0.0.1:8002",
			setEnv:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for each test
			viper.Reset()

			// Clean up environment variable before test
			_ = os.Unsetenv(tt.envKey)

			// Set environment variable if required
			if tt.setEnv {
				err := os.Setenv(tt.envKey, tt.envValue)
				if err != nil {
					t.Fatalf("Failed to set environment variable: %v", err)
				}
				// Clean up after test
				defer func() {
					_ = os.Unsetenv(tt.envKey)
				}()
			}

			// Enable automatic environment variable binding
			viper.AutomaticEnv()

			// Bind the environment variable to the viper key if different
			if tt.envKey != tt.viperKey {
				_ = viper.BindEnv(tt.viperKey, tt.envKey)
			}

			// Set the default value
			viper.SetDefault(tt.viperKey, tt.defaultValue)

			// Get the configuration value
			result := viper.GetString(tt.viperKey)

			// Verify the result
			if result != tt.expected {
				t.Errorf("viper.GetString(%q) = %q, want %q (env %s=%q)",
					tt.viperKey, result, tt.expected, tt.envKey, tt.envValue)
			}
		})
	}
}

// TestNonClowderConfigurationLoads verifies that configuration loads correctly
// when CLOWDER_ENABLED is false and environment variables are used.
func TestNonClowderConfigurationLoads(t *testing.T) {
	// Set CLOWDER_ENABLED to false
	_ = os.Setenv("CLOWDER_ENABLED", "false")
	defer func() {
		_ = os.Unsetenv("CLOWDER_ENABLED")
	}()

	// Set some custom environment variables
	testEnvVars := map[string]string{
		"DB_HOST":                 "test-postgres",
		"DB_PORT":                 "5432",
		"KAFKA_BOOTSTRAP_SERVERS": "test-kafka:9092",
		"DB_CA_CERT":              "test-ca-cert",
	}

	for key, value := range testEnvVars {
		_ = os.Setenv(key, value)
		defer func(k string) {
			_ = os.Unsetenv(k)
		}(key)
	}

	// Reset viper and reinitialize configuration
	viper.Reset()
	cfg = nil

	// This should trigger initConfig() with CLOWDER_ENABLED=false
	config := GetConfig()

	// Verify configuration loaded successfully
	if config == nil {
		t.Fatal("GetConfig() returned nil")
		return
	}

	// Verify environment variables were applied
	if config.DBHost != "test-postgres" {
		t.Errorf("DBHost = %q, want %q", config.DBHost, "test-postgres")
	}

	if config.DBPort != "5432" {
		t.Errorf("DBPort = %q, want %q", config.DBPort, "5432")
	}

	if config.KafkaBootstrapServers != "test-kafka:9092" {
		t.Errorf("KafkaBootstrapServers = %q, want %q", config.KafkaBootstrapServers, "test-kafka:9092")
	}

	if config.DBCACert != "test-ca-cert" {
		t.Errorf("DBCACert = %q, want %q", config.DBCACert, "test-ca-cert")
	}
}
