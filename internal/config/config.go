package config

import (
	"errors"

	"github.com/spf13/viper"
)

type Config struct {
	Addr         string
	DatabasePath string
	Token        string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("ID_LEDGER")
	v.AutomaticEnv()
	v.SetDefault("addr", ":3000")
	v.SetDefault("database_path", "id-ledger.db")

	config := Config{
		Addr:         v.GetString("addr"),
		DatabasePath: v.GetString("database_path"),
		Token:        v.GetString("token"),
	}
	if config.Token == "" {
		return Config{}, errors.New("ID_LEDGER_TOKEN is required")
	}
	return config, nil
}
