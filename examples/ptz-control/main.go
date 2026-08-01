// Command ptz-control pans a channel's PTZ left for two seconds, then stops.
//
// Configure via env vars: HIK_HOST, HIK_PORT (default 8000), HIK_USER,
// HIK_PASS, HIK_CHANNEL (default 1).
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/elyor04/go-hikvision-sdk/hikvision"
)

func main() {
	dev, err := hikvision.Login(hikvision.LoginOptions{
		Address:  requireEnv("HIK_HOST"),
		Port:     envPort("HIK_PORT", 8000),
		Username: requireEnv("HIK_USER"),
		Password: requireEnv("HIK_PASS"),
	})
	if err != nil {
		log.Fatalf("login: %v", err)
	}
	defer dev.Close()

	channel := int32(envInt("HIK_CHANNEL", 1))

	fmt.Println("panning left...")
	if err := dev.PTZControl(channel, hikvision.PTZPanLeft, false, 40); err != nil {
		log.Fatalf("ptz start: %v", err)
	}
	time.Sleep(2 * time.Second)
	if err := dev.PTZControl(channel, hikvision.PTZPanLeft, true, 40); err != nil {
		log.Fatalf("ptz stop: %v", err)
	}
	fmt.Println("stopped")
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing required env var %s", name)
	}
	return v
}

func envPort(name string, def uint16) uint16 {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	var port uint16
	if _, err := fmt.Sscanf(v, "%d", &port); err != nil {
		log.Fatalf("invalid %s: %v", name, err)
	}
	return port
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		log.Fatalf("invalid %s: %v", name, err)
	}
	return n
}
