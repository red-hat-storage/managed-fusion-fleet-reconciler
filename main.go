package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/db"
	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/forman"
	"github.com/red-hat-storage/managed-fusion-fleet-reconciler/pkg/reconciler"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type config struct {
	DB struct {
		Host     string            `yaml:"host"`
		Port     int               `yaml:"port"`
		User     string            `yaml:"user"`
		Password string            `yaml:"password"`
		Name     string            `yaml:"name"`
		Tables   map[string]string `yaml:"tables"`
	} `yaml:"db"`
	Reconcile struct {
		Concurrency int `yaml:"concurrency"`
	} `yaml:"reconcile"`
}

func loadAndValidateConfig(filePath string) (*config, error) {
	// parse configuration from yaml file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	cfg := &config{}
	if err := yaml.Unmarshal([]byte(data), cfg); err != nil {
		return nil, err
	}
	if cfg.DB.Host == "" {
		return nil, fmt.Errorf("config.db.host is not set")
	}
	if cfg.DB.Port == 0 {
		return nil, fmt.Errorf("config.db.port is not set")
	}
	if cfg.DB.User == "" {
		return nil, fmt.Errorf("config.db.user is not set")
	}
	if cfg.DB.Password == "" {
		return nil, fmt.Errorf("config.db.password is not set")
	}
	if cfg.DB.Name == "" {
		return nil, fmt.Errorf("config.db.name is not set")
	}
	if len(cfg.DB.Tables) == 0 {
		return nil, fmt.Errorf("config.db.tables is not set")
	}
	if cfg.Reconcile.Concurrency == 0 {
		return nil, fmt.Errorf("config.reconcile.concurrency is not set")
	}
	return cfg, nil
}

const configFileEnvVarName = "FLEET_RECONCILER_CONFIG"

func main() {
	ctx := context.Background()

	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevel(),
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	ZapLogger, err := zapConfig.Build()
	if err != nil {
		fmt.Printf("failed to create logger: %v\n", err)
		os.Exit(1)
	}
	log := zapr.NewLogger(ZapLogger)

	configFilePath := os.Getenv(configFileEnvVarName)
	if configFilePath == "" {
		log.Error(fmt.Errorf("%q environment variable not set", configFileEnvVarName), "failed to load configuration file path")
		os.Exit(1)
	}
	// parse configuration from yaml file
	conf, err := loadAndValidateConfig(configFilePath)
	if err != nil {
		log.Error(err, "failed to load configuration")
		os.Exit(1)
	}

	connString := db.GetConnectionString(conf.DB.Host, conf.DB.User, conf.DB.Password, conf.DB.Name, conf.DB.Port)

	dbClient, err := db.NewClient(ctx, connString, conf.DB.Tables)
	if err != nil {
		log.Error(err, "failed to create database client")
		os.Exit(1)
	}
	defer dbClient.Close(ctx)

	reqChan := forman.GoForman(log, conf.Reconcile.Concurrency,
		func(req forman.Request) forman.Result {
			return reconciler.Reconcile(log, dbClient, req)
		},
	)

	if err := dbClient.OnProvider(ctx, log, true, func(provideName string) {
		req := forman.Request{}
		req.Name = provideName
		reqChan <- req
	}); err != nil {
		log.Error(err, "failed to register provider listener")
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal to shutdown
	<-sigChan
	log.Info("Received signal, shutting down")
}
