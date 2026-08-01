// Command realplay-dump streams raw compressed video from a channel and
// writes every StreamStdVideoData buffer to stdout, so it can be piped
// straight into ffmpeg, e.g.:
//
//	go run ./examples/realplay-dump > /dev/null &   # (see README for a real ffmpeg pipeline)
//
// Configure via env vars: HIK_HOST, HIK_PORT (default 8000), HIK_USER,
// HIK_PASS, HIK_CHANNEL (default 1). Stop with Ctrl+C.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

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

	channel := envInt("HIK_CHANNEL", 1)
	stream, err := dev.RealPlay(ctx, int32(channel), hikvision.MainStream)
	if err != nil {
		log.Fatalf("realplay: %v", err)
	}
	defer stream.Close()

	fmt.Fprintf(os.Stderr, "streaming channel %d - Ctrl+C to stop\n", channel)
	var total int64
	for frame := range stream.Frames() {
		if frame.Type != hikvision.StreamStdVideoData {
			continue
		}
		n, err := os.Stdout.Write(frame.Data)
		total += int64(n)
		if err != nil {
			log.Fatalf("write: %v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "stream ended, wrote %d bytes\n", total)
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
