package config

import (
	"os"
	"testing"
)

func TestGetEnvWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue string
		expected     string
		setEnv       bool
	}{
		{
			name:         "returns environment variable when set",
			envKey:       "TEST_ENV_VAR",
			envValue:     "test_value",
			defaultValue: "default_value",
			expected:     "test_value",
			setEnv:       true,
		},
		{
			name:         "returns default when environment variable not set",
			envKey:       "UNSET_ENV_VAR",
			envValue:     "",
			defaultValue: "default_value",
			expected:     "default_value",
			setEnv:       false,
		},
		{
			name:         "returns default when environment variable is empty string",
			envKey:       "EMPTY_ENV_VAR",
			envValue:     "",
			defaultValue: "default_value",
			expected:     "default_value",
			setEnv:       true,
		},
		{
			name:         "handles empty default value",
			envKey:       "ANOTHER_UNSET_VAR",
			envValue:     "",
			defaultValue: "",
			expected:     "",
			setEnv:       false,
		},
		{
			name:         "handles special characters in environment variable",
			envKey:       "SPECIAL_CHARS_VAR",
			envValue:     "value-with_special.chars:123",
			defaultValue: "default",
			expected:     "value-with_special.chars:123",
			setEnv:       true,
		},
		{
			name:         "handles spaces in environment variable",
			envKey:       "SPACES_VAR",
			envValue:     "value with spaces",
			defaultValue: "default",
			expected:     "value with spaces",
			setEnv:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variable before test
			os.Unsetenv(tt.envKey)

			// Set environment variable if required
			if tt.setEnv {
				err := os.Setenv(tt.envKey, tt.envValue)
				if err != nil {
					t.Fatalf("Failed to set environment variable: %v", err)
				}
				// Clean up after test
				defer os.Unsetenv(tt.envKey)
			}

			// Test the function
			result := getEnvWithDefault(tt.envKey, tt.defaultValue)

			// Verify the result
			if result != tt.expected {
				t.Errorf("getEnvWithDefault(%q, %q) = %q, want %q",
					tt.envKey, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

// Benchmark test to ensure the function is efficient
func BenchmarkGetEnvWithDefault(b *testing.B) {
	// Set up test environment variable
	os.Setenv("BENCHMARK_VAR", "benchmark_value")
	defer os.Unsetenv("BENCHMARK_VAR")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test with existing env var
		getEnvWithDefault("BENCHMARK_VAR", "default")
		// Test with non-existing env var
		getEnvWithDefault("NON_EXISTING_VAR", "default")
	}
}
