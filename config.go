package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Domain string `toml:"domain"`
	DB     struct {
		Region   string `toml:"region"`
		Prefix   string `toml:"prefix"`
		Endpoint string `toml:"endpoint"`
		Debug    bool   `toml:"debug"`
	} `toml:"db"`
	Storage struct {
		Type              string `toml:"type"`
		FilesBucket       string `toml:"files_bucket"`
		UploadsBucket     string `toml:"uploads_bucket"`
		AccessKeyID       string `toml:"access_key_id"`
		AccessKeySecret   string `toml:"access_key_secret"`
		CloudflareAccount string `toml:"cloudflare_account"`
		Domain            string `toml:"domain"`
		Region            string `toml:"region"`
		Endpoint          string `toml:"endpoint"`
	} `toml:"storage"`
}

func readConfig(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config: %w", err)
	}
	err = toml.Unmarshal(raw, &cfg)
	return cfg, err
}
