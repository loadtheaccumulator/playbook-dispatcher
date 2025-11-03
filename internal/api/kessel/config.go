package kessel

import "github.com/spf13/viper"

// ConfigureDefaults sets default Kessel configuration values
func ConfigureDefaults(cfg *viper.Viper) {
	// Kessel connection settings
	cfg.SetDefault("kessel.impl", "mock")
	cfg.SetDefault("kessel.hostname", "kessel-relations")
	cfg.SetDefault("kessel.port", 9000)
	cfg.SetDefault("kessel.insecure", false)
	cfg.SetDefault("kessel.timeout", 10) // seconds

	// Feature flag to enable/disable Kessel
	cfg.SetDefault("kessel.enabled", false)
}

// NewKesselClientFromConfig creates a Kessel client based on configuration
// Returns mock client if kessel.impl is "mock", otherwise real client
func NewKesselClientFromConfig(cfg *viper.Viper) (KesselClient, error) {
	impl := cfg.GetString("kessel.impl")

	if impl == "mock" {
		// Mock client that allows all access
		return NewMockKesselClient(true), nil
	}

	// Real Kessel client
	return NewKesselClient(cfg)
}
