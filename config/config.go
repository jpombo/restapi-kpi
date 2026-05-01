package config

import (
	"log"
	"os"
	"strconv"
	"strings"

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

	log.Println(os.Getenv("DB_LOCAL"))

	// Garantir que a variável de ambiente seja lida
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "300"))
	ollamaTimeout, _ := strconv.Atoi(getEnv("OLLAMA_TIMEOUT", "600"))

	return &Config{
		DBHost:        getEnv("DB_HOST", ""),
		DBPort:        dbPort,
		DBUser:        getEnv("DB_USER", ""),
		DBPassword:    strings.ReplaceAll(os.Getenv("DB_LOCAL"), "_", "&"),
		DBName:        getEnv("DB_NAME", ""),
		OllamaURL:     getEnv("OLLAMA_URL", ""),
		OllamaModel:   getEnv("OLLAMA_MODEL", ""),
		OllamaTimeout: ollamaTimeout,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
