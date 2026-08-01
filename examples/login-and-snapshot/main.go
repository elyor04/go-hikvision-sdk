// Command login-and-snapshot logs into a device, prints its info, takes a
// JPEG snapshot of channel 1, and logs out.
//
// Configure via env vars: HIK_HOST, HIK_PORT (default 8000), HIK_USER,
// HIK_PASS.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/elyor04/go-hikvision-sdk/hikvision"
)

func main() {
	fmt.Println("HCNetSDK version:", hikvision.Version())

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

	fmt.Printf("logged in: %+v\n", dev.Info)

	jpeg, err := dev.CaptureJPEG(1, hikvision.JPEGBest)
	if err != nil {
		log.Fatalf("capture: %v", err)
	}
	if err := os.WriteFile("snapshot.jpg", jpeg, 0o644); err != nil {
		log.Fatalf("write snapshot: %v", err)
	}
	fmt.Printf("wrote snapshot.jpg (%d bytes)\n", len(jpeg))
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
