package main

import (
	"context"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v3"
)

func main() {
	PROJECT_ID := os.Getenv("GCLOUD_PROJECT_ID")
	if PROJECT_ID == "" {
		log.Fatal("GCLOUD_PROJECT_ID environment variable not set")
	}

	app := fiber.New()
	ctx := context.Background()

	client, err := firestore.NewClient(ctx, PROJECT_ID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	app.Get("/incidents", GetIncidents(client))

	app.Get("/incidents/:id", GetIncident(client))

	app.Post("/incidents", CreateIncident(client))

	app.Patch("/incidents/:id", UpdateIncident(client))

	app.Delete("/incidents/:id", DeleteIncident(client))

	app.Use(func(c fiber.Ctx) error {
		return nil
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Fatal(app.Listen(":" + port))
}
