package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Addr         string `envconfig:"ADDR" default:"127.0.0.1:8080"`
	DataDir      string `envconfig:"DATA_DIR" default:"./data"`
	StaticDir    string `envconfig:"STATIC_DIR" default:"./web/dist"`
	CookieSecure bool   `envconfig:"COOKIE_SECURE" default:"false"`
	SetupToken   string `envconfig:"SETUP_TOKEN"`
	LogLevel     string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat    string `envconfig:"LOG_FORMAT" default:"text"`
	LogFile      string `envconfig:"LOG_FILE"`
	DockerHost   string `envconfig:"DOCKER_HOST"`
	HostProc     string `envconfig:"HOST_PROC"`
	HostSys      string `envconfig:"HOST_SYS"`
	HostRoot     string `envconfig:"HOST_ROOT"`
	HostDisk     string `envconfig:"HOST_DISK"`
	HostName     string `envconfig:"HOST_NAME"`
}

func LoadConfig() (Config, error) {
	var cfg Config
	err := envconfig.Process("DASHBOARD", &cfg)
	if err == nil && (strings.TrimSpace(cfg.Addr) == "" || strings.TrimSpace(cfg.DataDir) == "" || strings.TrimSpace(cfg.StaticDir) == "") {
		err = fmt.Errorf("address, data directory and static directory must not be empty")
	}
	return cfg, err
}

func persistentSecret(path string) ([]byte, error) {
	if key, err := os.ReadFile(path); err == nil {
		key, err = base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(key)))
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid secret file: %s", filepath.Base(path))
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return persistentSecret(path)
	}
	if err != nil {
		return nil, err
	}
	_, writeErr := file.WriteString(base64.RawURLEncoding.EncodeToString(key) + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	return key, closeErr
}
