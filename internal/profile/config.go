package profile

import (
	"github.com/jmcampanini/brewkit/internal/config"
	"github.com/spf13/pflag"
)

// LoadConfig loads configuration and resolves the active profiles.
func LoadConfig(path string, flags *pflag.FlagSet) (config.Config, config.LoadReport, []string, error) {
	cfg, report, err := config.LoadWithReport(path, flags)
	if err != nil {
		return config.Config{}, config.LoadReport{}, nil, err
	}
	resolved, err := Resolve(cfg)
	if err != nil {
		return config.Config{}, config.LoadReport{}, nil, err
	}
	return cfg, report, resolved, nil
}
