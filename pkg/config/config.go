package config

import (
	"os"
)

type Config struct {
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	DBSSLMode           string
	ServerPort          string
	JWTSecret           string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func NewConfig() *Config {
	return &Config{
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "acareca"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
	}
}

func (c *Config) GetDBHost() string {
	return c.DBHost
}

func (c *Config) GetDBPort() string {
	return c.DBPort
}

func (c *Config) GetDBUser() string {
	return c.DBUser
}

func (c *Config) GetDBPassword() string {
	return c.DBPassword
}

func (c *Config) GetDBName() string {
	return c.DBName
}

func (c *Config) GetDBSSLMode() string {
	return c.DBSSLMode
}

func (c *Config) GetServerPort() string {
	return c.ServerPort
}

func (c *Config) GetJWTSecret() string {
	return c.JWTSecret
}
