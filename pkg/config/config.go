package config

import (
	"os"
)

type Config struct {
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	ServerPort         string
	JWTSecret          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

func getEnv(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return val
}

func NewConfig() *Config {
	return &Config{
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", "postgres"),
		DBName:             getEnv("DB_NAME", "acareca"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/oauth"),
	}
}

func (c *Config) GetDBHost() string {
	if c.DBHost == "" {
		return os.Getenv("DB_HOST")
	}
	return c.DBHost
}

func (c *Config) GetDBPort() string {
	if c.DBPort == "" {
		return os.Getenv("DB_PORT")
	}
	return c.DBPort
}

func (c *Config) GetDBUser() string {
	if c.DBUser == "" {
		return os.Getenv("DB_USER")
	}
	return c.DBUser
}

func (c *Config) GetDBPassword() string {
	if c.DBPassword == "" {
		return os.Getenv("DB_PASSWORD")
	}
	return c.DBPassword
}

func (c *Config) GetDBName() string {
	if c.DBName == "" {
		return os.Getenv("DB_NAME")
	}
	return c.DBName
}

func (c *Config) GetDBSSLMode() string {
	if c.DBSSLMode == "" {
		return os.Getenv("DB_SSLMODE")
	}
	return c.DBSSLMode
}

func (c *Config) GetServerPort() string {
	if c.ServerPort == "" {
		return os.Getenv("SERVER_PORT")
	}
	return c.ServerPort
}

func (c *Config) GetJWTSecret() string {
	if c.JWTSecret == "" {
		return os.Getenv("JWT_SECRET")
	}
	return c.JWTSecret
}
