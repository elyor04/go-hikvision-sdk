// Command playback-download searches for recordings on a channel over the
// last hour and downloads the first one found to a local file.
//
// Configure via env vars: HIK_HOST, HIK_PORT (default 8000), HIK_USER,
// HIK_PASS, HIK_CHANNEL (default 1).
package main

import (
	"context"
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
	stop := time.Now()
	start := stop.Add(-1 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	recordings, err := dev.FindRecordings(ctx, channel, start, stop)
	if err != nil {
		log.Fatalf("FindRecordings: %v", err)
	}
	if len(recordings) == 0 {
		fmt.Println("no recordings found in the last hour")
		return
	}
	fmt.Printf("found %d recording(s); downloading the first\n", len(recordings))

	rec := recordings[0]
	outPath := "download.mp4"
	dl, err := dev.Download(channel, rec.Start, rec.Stop, outPath)
	if err != nil {
		log.Fatalf("Download: %v", err)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer waitCancel()
	if err := dl.Wait(waitCtx); err != nil {
		log.Fatalf("Wait: %v", err)
	}
	fmt.Println("downloaded to", outPath)

	if info, err := os.Stat(outPath); err == nil {
		fmt.Printf("file size: %d bytes\n", info.Size())
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
