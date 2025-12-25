package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Storage  StorageConfig
}

type ServerConfig struct {
	Host string
	Port int
	Mode string // debug, release, test
}

type DatabaseConfig struct {
	SQLitePath string
}

type JWTConfig struct {
	Secret     string
	ExpireHour time.Duration
}

type StorageConfig struct {
	BasePath string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	viper.SetDefault("database.sqlite_path", "./backup.db")

	viper.SetDefault("jwt.secret", "change-this-secret-in-production")
	viper.SetDefault("jwt.expire_hour", 24)

	viper.SetDefault("storage.base_path", "./storage")

	// Environment variables
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: viper.GetString("server.host"),
			Port: viper.GetInt("server.port"),
			Mode: viper.GetString("server.mode"),
		},
		Database: DatabaseConfig{
			SQLitePath: viper.GetString("database.sqlite_path"),
		},
		JWT: JWTConfig{
			Secret:     viper.GetString("jwt.secret"),
			ExpireHour: time.Duration(viper.GetInt("jwt.expire_hour")),
		},
		Storage: StorageConfig{
			BasePath: viper.GetString("storage.base_path"),
		},
	}

	return cfg, nil
}
