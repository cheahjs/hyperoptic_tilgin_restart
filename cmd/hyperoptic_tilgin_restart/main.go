package main

import (
	"flag"
	"os"

	"github.com/cheahjs/hyperoptic_tilgin_restart/internal/tilgin"
	"go.uber.org/zap"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging")
	username := flag.String("username", "", "Username to login as")
	routerHost := flag.String("host", "http://192.168.1.1", "Router host")
	password := os.Getenv("ROUTER_PASSWORD")

	flag.Parse()

	var logger *zap.Logger
	if *debug {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}

	if *username == "" {
		logger.Fatal("Username is not set")
	}
	if *routerHost == "" {
		logger.Fatal("Router host is not set")
	}
	if password == "" {
		logger.Fatal("Password is not set")
	}

	restarter := tilgin.NewRestarter(logger.Sugar(), *username, password, *routerHost)
	err := restarter.Restart()
	if err != nil {
		logger.Sugar().Fatalw("Failed to restart",
			"error", err)
		os.Exit(1)
	}
}
