package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/rogeecn/id-ledger/internal/api"
	"github.com/rogeecn/id-ledger/internal/config"
	"github.com/rogeecn/id-ledger/internal/database"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	connection, err := database.Open(settings.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	server := api.New(connection, settings.Token)
	log.Fatal(server.App().Listen(settings.Addr, fiber.ListenConfig{DisableStartupMessage: true}))
}
