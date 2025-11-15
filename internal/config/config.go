package config

import (
	"log/slog"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort                string `env:"PORT" envDefault:"8080"`
	NameFileAllTasks          string `env:"ALL_TASKS_FILE" envDefault:"storage/AllTasks.json"`
	NameFileProcessTasksLinks string `env:"PROCESS_LINKS_FILE" envDefault:"storage/ProcessTasksLinks.json"`
	NameFileProcessTasksNums  string `env:"PROCESS_NUMS_FILE" envDefault:"storage/ProcessTasksNums.json"`
}

func MustLoad() *Config {

	if err := godotenv.Load(); err != nil {
		slog.Debug("Failed to load .env file", "error", err)
	} else {
		slog.Info("Loaded configuration from .env file")
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Failed to parse environment variables", "error", err)
		panic("configuration error: " + err.Error())
	}

	return &cfg
}
