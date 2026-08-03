// Command anpr-listener subscribes to a device's alarm stream and prints
// every decoded license-plate (ANPR) event, saving each plate's snapshot
// image to disk if present.
//
// Configure via env vars: HIK_HOST, HIK_PORT (default 8000), HIK_USER,
// HIK_PASS. Stop with Ctrl+C.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	plates, err := dev.PlateEvents(ctx)
	if err != nil {
		log.Fatalf("PlateEvents: %v", err)
	}

	fmt.Println("listening for ANPR events - Ctrl+C to stop")
	for ev := range plates {
		zoneName, zoneOffset := ev.CaptureTime.Zone()
		fmt.Printf("[plate] %q confidence=%d speed=%dkm/h at %s (zone=%s offset=%ds, UTC=%s)\n",
			ev.License, ev.Confidence, ev.SpeedKMH, ev.CaptureTime.Format(time.RFC3339),
			zoneName, zoneOffset, ev.CaptureTime.UTC().Format(time.RFC3339))

		if len(ev.SceneImage) > 0 {
			name := fmt.Sprintf("plate-%s-scene.jpg", ev.CaptureTime.Format("20060102-150405"))
			if err := os.WriteFile(name, ev.SceneImage, 0o644); err != nil {
				log.Printf("write %s: %v", name, err)
			} else {
				fmt.Println("  saved", name)
			}
		}
		if len(ev.PlateImage) > 0 {
			name := fmt.Sprintf("plate-%s-crop.jpg", ev.CaptureTime.Format("20060102-150405"))
			if err := os.WriteFile(name, ev.PlateImage, 0o644); err != nil {
				log.Printf("write %s: %v", name, err)
			} else {
				fmt.Println("  saved", name)
			}
		}
	}
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
