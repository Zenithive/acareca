package config

import (
	"fmt"
	"os"
	"strconv"
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
	AllowedOrigins     string
	StripeSecretKey    string
	FrontendURL        string

	// File Upload Configuration
	FileUploadMaxSize         int64
	FileUploadAllowedTypes    string
	FileUploadStorageProvider string
	FileUploadLocalPath       string

	// R2 Configuration
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketName      string
	R2PublicURL       string
}

func getEnv(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	return val
}

func getEnvInt64(key string, fallback int64) int64 {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}

	intVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return fallback
	}
	return intVal
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
		AllowedOrigins:     getEnv("ALLOWED_ORIGINS", ""),
		StripeSecretKey:    getEnv("STRIPE_SECRET_KEY", "ETC"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),

		// File Upload Configuration
		FileUploadMaxSize:         getEnvInt64("FILE_UPLOAD_MAX_SIZE", 10485760), // 10MB default
		FileUploadAllowedTypes:    getEnv("FILE_UPLOAD_ALLOWED_TYPES", "image/jpeg,image/png,image/gif,application/pdf,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,text/csv"),
		FileUploadStorageProvider: getEnv("FILE_UPLOAD_STORAGE_PROVIDER", "local"),
		// FileUploadLocalPath:       getEnv("FILE_UPLOAD_LOCAL_PATH", "./uploads"),

		// R2 Configuration
		R2AccountID:       getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2BucketName:      getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:       getEnv("R2_PUBLIC_URL", ""),
	}
}

func (c *Config) GetBaseURL() (string, error) {
	switch c.Env {
	case "":
		return "", fmt.Errorf("environment not set")
	case "production":
		return c.DevUrl, nil
	default:
		return c.LocalUrl, nil
	}
}
