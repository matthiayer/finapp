package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultListenAddress = "127.0.0.1:8080"
	defaultDatabasePath  = "data/finapp.db"
)

type Config struct {
	ListenAddress string
	DatabasePath  string
}

func Load() (Config, error) {
	listenAddress := strings.TrimSpace(os.Getenv("FINAPP_LISTEN_ADDR"))
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}
	if err := validateListenAddress(listenAddress); err != nil {
		return Config{}, err
	}

	databasePath := strings.TrimSpace(os.Getenv("FINAPP_DB_PATH"))
	if databasePath == "" {
		databasePath = defaultDatabasePath
	}

	return Config{
		ListenAddress: listenAddress,
		DatabasePath:  databasePath,
	}, nil
}

func validateListenAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("FINAPP_LISTEN_ADDR must be a host and port: %w", err)
	}
	if host == "" {
		return fmt.Errorf("FINAPP_LISTEN_ADDR must name an interface address")
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("FINAPP_LISTEN_ADDR port must be between 1 and 65535")
	}
	return nil
}
