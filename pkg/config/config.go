package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/KyberNetwork/logger"
	"github.com/spf13/viper"

	"github.com/KyberNetwork/service-framework/pkg/server/grpcserver"
)

type Config struct {
	Server grpcserver.Config
}

//go:embed default.yaml
var defaultConfig []byte

func LoadConfig(configPath string) (Config, error) {
	cfg := Config{}
	if configPath != "" {
		viper.SetConfigFile(configPath)
	}
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		logger.Warnf("readInConfig error with %v", err)
		err := viper.ReadConfig(bytes.NewBuffer(defaultConfig))
		if err != nil {
			log.Fatalf("Failed to read viper config %v", err)
		}
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.Unmarshal(&cfg); err != nil {
		log.Printf("failed to unmarshal config %v\n", err)
		return Config{}, err
	}
	bytesCfg, _ := json.Marshal(cfg)
	fmt.Println("cfg", string(bytesCfg))
	return cfg, nil
}
