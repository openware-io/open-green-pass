package config

import "testing"

func TestTrustedDSNRequiresDedicatedConnectionInProduction(t *testing.T) {
	c := &Config{Env: "prod", DBDSN: "primary"}
	if _, err := c.TrustedDSN(); err == nil {
		t.Fatal("production accepted a missing trusted DSN")
	}
}

func TestTrustedDSNUsesConfiguredValue(t *testing.T) {
	c := &Config{Env: "prod", DBDSN: "primary", TrustedDBDSN: "trusted"}
	got, err := c.TrustedDSN()
	if err != nil || got != "trusted" {
		t.Fatalf("TrustedDSN() = %q, %v", got, err)
	}
}

func TestTrustedDSNRejectsSharedProductionCredential(t *testing.T) {
	c := &Config{Env: "prod", DBDSN: "postgres://app@db/gp", TrustedDBDSN: " postgres://app@db/gp "}
	if _, err := c.TrustedDSN(); err == nil {
		t.Fatal("production accepted the primary application credential as trusted writer")
	}
}
