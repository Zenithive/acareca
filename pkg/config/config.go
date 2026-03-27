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
	JWTRefreshSecret   string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	ResendAPIKey       string
	Env                string
	DevUrl             string
	LocalUrl           string
}

func getEnv(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
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
		JWTRefreshSecret:   getEnv("JWT_REFRESH_SECRET", "change-me-refresh"),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/oauth"),
		ResendAPIKey:       getEnv("RESEND_API_KEY", ""),
		Env:                getEnv("ENV", "local"),
		DevUrl:             getEnv("DEV_API_URL", "https://acareca-bam8.onrender.com"),
		LocalUrl:           getEnv("LOCAL_API_URl", "http://localhost:5173"),
	}
}
