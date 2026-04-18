package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	OllamaURL     string
	OllamaModel   string
	OllamaTimeout int // Timeout em segundos
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, usando variáveis de ambiente")
	}

	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "3306"))
	ollamaTimeout, _ := strconv.Atoi(getEnv("OLLAMA_TIMEOUT", "300"))

	return &Config{
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        dbPort,
		DBUser:        getEnv("DB_USER", "root"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", "kpi_db"),
		OllamaURL:     getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "kpi-requirement-generator"),
		OllamaTimeout: ollamaTimeout,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
