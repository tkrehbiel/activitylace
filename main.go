// Starts an http server to respond to ActivityPub requests.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"time"

	"github.com/tkrehbiel/activitylace/server"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

func readConfig(filename string) server.Config {
	var cfg server.Config
	b, err := os.ReadFile(filename)
	if err != nil {
		telemetry.Error(err, "opening config [%s]", filename)
	} else {
		c, err := server.ReadConfig(b)
		if err != nil {
			telemetry.Error(err, "parsing config [%s]", filename)
		}
		cfg = c
	}

	return cfg
}

func main() {
	configFile := flag.String("config", "config.json", "config json file")
	host := flag.String("host", "", "this hostname")
	pubCert := flag.String("cert", "", "public certificate")
	privCert := flag.String("key", "", "private key")
	port := flag.Int("port", 0, "listen port")

	flag.Parse()

	telemetry.Log("starting activitylace")

	cfg := readConfig(*configFile)
	if *host != "" {
		cfg.Server.HostName = *host
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *pubCert != "" {
		cfg.Server.Certificate = *pubCert
	}
	if *privCert != "" {
		cfg.Server.PrivateKey = *privCert
	}

	svc := server.NewService(cfg)

	// Startup the service to listen for http requests
	svc.Start(context.Background())

	// Wait for ^C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	telemetry.Log("stopping activitylace")

	// Shut down the service
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	svc.Stop(ctx)
	telemetry.Log("stopped activitylace cleanly")
}
