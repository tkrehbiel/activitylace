// Starts an http server to respond to ActivityPub requests.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/tkrehbiel/activitylace/server"
)

func validUser(username string) bool {
	switch username {
	case "blog":
		return true
	}
	return false
}

const activityPubMimeType = "application/activity+json"
const activityStreamMimeType = `application/ld+json`

func readConfig(filename string) server.Config {
	var cfg server.Config
	b, err := os.ReadFile(filename)
	if err != nil {
		log.Println(err)
	} else {
		c, err := server.ReadConfig(b)
		if err != nil {
			log.Println(err)
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

	srv := server.NewService(cfg)

	go func() {
		defer srv.Close()
		err := srv.ListenAndServe()
		if err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	srv.Server.Shutdown(ctx)
	log.Println("server stopping")
	os.Exit(0)
}
