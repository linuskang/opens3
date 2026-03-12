package config

import (
	"os"
	"strconv"
)

// Config holds all server configuration.
type Config struct {
	// APIPort is the HTTP port for the S3-compatible bucket API.
	APIPort int
	// UIPort is the HTTP port for the web UI.
	UIPort int
	// DataDir is the root directory for storing data.
	DataDir string
	// AccessKey is the AWS-style access key for authentication.
	AccessKey string
	// SecretKey is the AWS-style secret key for authentication.
	SecretKey string
	// Region is the S3 region name (cosmetic, returned in responses).
	Region string
	// UIEnabled controls whether the web UI is served.
	UIEnabled bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	apiPort := 9001
	if v := os.Getenv("OPENS3_API_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			apiPort = p
		}
	} else if v := os.Getenv("OPENS3_PORT"); v != "" {
		// OPENS3_PORT is a legacy alias for OPENS3_API_PORT.
		if p, err := strconv.Atoi(v); err == nil {
			apiPort = p
		}
	}

	uiPort := 9000
	if v := os.Getenv("OPENS3_UI_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			uiPort = p
		}
	}

	dataDir := os.Getenv("OPENS3_DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	accessKey := os.Getenv("OPENS3_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "minioadmin"
	}

	secretKey := os.Getenv("OPENS3_SECRET_KEY")
	if secretKey == "" {
		secretKey = "minioadmin"
	}

	region := os.Getenv("OPENS3_REGION")
	if region == "" {
		region = "us-east-1"
	}

	uiDisabled := os.Getenv("OPENS3_UI_DISABLED") == "true"

	return &Config{
		APIPort:   apiPort,
		UIPort:    uiPort,
		DataDir:   dataDir,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
		UIEnabled: !uiDisabled,
	}
}
