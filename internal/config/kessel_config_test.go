package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func resetTestEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
	viper.Reset()
	ResetConfig()
}

var kesselEnvKeys = []string{
	"AUTHORIZATION_BACKEND",
	"KESSEL_RELATIONS_URL",
	"KESSEL_RELATIONS_CA_PATH",
	"KESSEL_INVENTORY_URL",
	"KESSEL_INVENTORY_CA_PATH",
	"SPICEDB_PRESHARED_KEY",
	"RBAC_ENABLE",
	"CLOWDER_ENABLED",
}

func TestAuthorizationBackendDefault(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")

	c := GetConfig()
	if c.AuthorizationBackend != "rbac" {
		t.Errorf("UT-CFG-BACKEND-001: AuthorizationBackend = %q, want %q", c.AuthorizationBackend, "rbac")
	}
}

func TestAuthorizationBackendOverride(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("AUTHORIZATION_BACKEND", "kessel")

	c := GetConfig()
	if c.AuthorizationBackend != "kessel" {
		t.Errorf("UT-CFG-BACKEND-002: AuthorizationBackend = %q, want %q", c.AuthorizationBackend, "kessel")
	}
}

func TestKesselRelationsURLDefault(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")

	c := GetConfig()
	if c.KesselRelationsURL != "localhost:9000" {
		t.Errorf("UT-CFG-BACKEND-003: KesselRelationsURL = %q, want %q", c.KesselRelationsURL, "localhost:9000")
	}
}

func TestKesselRelationsURLOverride(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("KESSEL_RELATIONS_URL", "kessel.example.com:443")

	c := GetConfig()
	if c.KesselRelationsURL != "kessel.example.com:443" {
		t.Errorf("UT-CFG-BACKEND-004: KesselRelationsURL = %q, want %q", c.KesselRelationsURL, "kessel.example.com:443")
	}
}

func TestKesselRelationsCAPathDefault(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")

	c := GetConfig()
	if c.KesselRelationsCAPath != "" {
		t.Errorf("UT-CFG-BACKEND-005: KesselRelationsCAPath = %q, want empty (system trust store)", c.KesselRelationsCAPath)
	}
}

func TestKesselRelationsCAPathOverride(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("KESSEL_RELATIONS_CA_PATH", "/etc/pki/tls/certs/kessel-ca.crt")

	c := GetConfig()
	if c.KesselRelationsCAPath != "/etc/pki/tls/certs/kessel-ca.crt" {
		t.Errorf("UT-CFG-BACKEND-006: KesselRelationsCAPath = %q, want %q", c.KesselRelationsCAPath, "/etc/pki/tls/certs/kessel-ca.crt")
	}
}

func TestSpiceDBPresharedKeyDefault(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")

	c := GetConfig()
	if c.SpiceDBPresharedKey != "" {
		t.Errorf("UT-CFG-BACKEND-007: SpiceDBPresharedKey = %q, want empty", c.SpiceDBPresharedKey)
	}
}

func TestSpiceDBPresharedKeyOverride(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("SPICEDB_PRESHARED_KEY", "my-secret-key")

	c := GetConfig()
	if c.SpiceDBPresharedKey != "my-secret-key" {
		t.Errorf("UT-CFG-BACKEND-008: SpiceDBPresharedKey = %q, want %q", c.SpiceDBPresharedKey, "my-secret-key")
	}
}

func TestRBACEnabledDefault(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")

	c := GetConfig()
	if c.RBACEnabled != false {
		t.Errorf("UT-CFG-BACKEND-009: RBACEnabled = %v, want false", c.RBACEnabled)
	}
}

func TestRBACEnabledWithKessel(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("RBAC_ENABLE", "true")
	_ = os.Setenv("AUTHORIZATION_BACKEND", "kessel")

	c := GetConfig()
	if c.RBACEnabled != true {
		t.Errorf("UT-CFG-BACKEND-010: RBACEnabled = %v, want true", c.RBACEnabled)
	}
	if c.AuthorizationBackend != "kessel" {
		t.Errorf("UT-CFG-BACKEND-010: AuthorizationBackend = %q, want %q", c.AuthorizationBackend, "kessel")
	}
}

func TestRBACDisabledOverridesBackend(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("RBAC_ENABLE", "false")
	_ = os.Setenv("AUTHORIZATION_BACKEND", "kessel")

	c := GetConfig()
	if c.RBACEnabled != false {
		t.Errorf("UT-CFG-BACKEND-011: RBACEnabled = %v, want false", c.RBACEnabled)
	}
	if c.AuthorizationBackend != "kessel" {
		t.Errorf("UT-CFG-BACKEND-011: AuthorizationBackend = %q, want %q (set but RBACEnabled=false makes it irrelevant)", c.AuthorizationBackend, "kessel")
	}
}

// --- Inventory API config tests ---.

// UT-CFG-INV-001: KESSEL_INVENTORY_URL defaults to localhost:9081.
func TestKesselInventoryURLDefault(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")

	c := GetConfig()
	if c.KesselInventoryURL != "localhost:9081" {
		t.Errorf("UT-CFG-INV-001: KesselInventoryURL = %q, want %q", c.KesselInventoryURL, "localhost:9081")
	}
}

// UT-CFG-INV-002: KESSEL_INVENTORY_URL env override is read.
func TestKesselInventoryURLOverride(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("KESSEL_INVENTORY_URL", "inventory.example.com:443")

	c := GetConfig()
	if c.KesselInventoryURL != "inventory.example.com:443" {
		t.Errorf("UT-CFG-INV-002: KesselInventoryURL = %q, want %q", c.KesselInventoryURL, "inventory.example.com:443")
	}
}

// UT-CFG-INV-003: KESSEL_INVENTORY_CA_PATH env override is read.
func TestKesselInventoryCAPathOverride(t *testing.T) {
	resetTestEnv(t, kesselEnvKeys...)
	t.Cleanup(func() { resetTestEnv(t, kesselEnvKeys...) })

	_ = os.Setenv("CLOWDER_ENABLED", "false")
	_ = os.Setenv("KESSEL_INVENTORY_CA_PATH", "/etc/pki/tls/certs/inventory-ca.crt")

	c := GetConfig()
	if c.KesselInventoryCAPath != "/etc/pki/tls/certs/inventory-ca.crt" {
		t.Errorf("UT-CFG-INV-003: KesselInventoryCAPath = %q, want %q", c.KesselInventoryCAPath, "/etc/pki/tls/certs/inventory-ca.crt")
	}
}
