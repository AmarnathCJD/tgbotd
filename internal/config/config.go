package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	APIID       int32
	APIHash     string
	HTTPAddr    string
	DataDir     string
	LogLevel    string
	TestDC      bool
	LocalMode   bool
	MaxUpload   int64
	MaxDownload int64
	StatsAddr   string
}

func Load() (*Config, error) {
	c := &Config{
		HTTPAddr:    envOr("TGBOTD_HTTP_ADDR", "127.0.0.1:8081"),
		DataDir:     envOr("TGBOTD_DATA_DIR", "./data"),
		LogLevel:    envOr("TGBOTD_LOG_LEVEL", "info"),
		APIHash:     envOr("TGBOTD_API_HASH", ""),
		MaxUpload:   2 * 1024 * 1024 * 1024,
		MaxDownload: 2 * 1024 * 1024 * 1024,
		StatsAddr:   envOr("TGBOTD_STATS_ADDR", ""),
	}
	if v := os.Getenv("TGBOTD_API_ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("TGBOTD_API_ID: %w", err)
		}
		c.APIID = int32(n)
	}
	if strings.EqualFold(os.Getenv("TGBOTD_TEST_DC"), "true") {
		c.TestDC = true
	}
	if strings.EqualFold(os.Getenv("TGBOTD_LOCAL"), "true") {
		c.LocalMode = true
	}
	if c.APIID == 0 {
		c.APIID = 2040
	}
	if c.APIHash == "" {
		c.APIHash = "b18441a1ff607e10a989891a5462e627"
	}
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}
	if c.APIID == 0 || c.APIHash == "" {
		return nil, errors.New("missing api_id/api_hash")
	}
	return c, nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
