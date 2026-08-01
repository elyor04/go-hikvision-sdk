// Command alarm-listener subscribes to a device's alarm/event stream and
// prints every event it receives until interrupted.
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

	if err := hikvision.OnException(func(t hikvision.ExceptionType, userID, handle int32) {
		fmt.Printf("[exception] type=0x%x userID=%d handle=%d\n", uint32(t), userID, handle)
	}); err != nil {
		log.Fatalf("OnException: %v", err)
	}

	events, err := dev.Alarms(ctx)
	if err != nil {
		log.Fatalf("Alarms: %v", err)
	}

	fmt.Println("listening for alarms - Ctrl+C to stop")
	for ev := range events {
		fmt.Printf("[event] command=0x%x deviceIP=%q raw=%d bytes plate=%v\n",
			uint32(ev.Command), ev.Alarmer.DeviceIP, len(ev.Raw), ev.Plate != nil)
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
