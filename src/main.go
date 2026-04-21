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
	app.Use(func(c fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, api-key, X-IncidentTNX-Id")
		if c.Method() == "OPTIONS" {
			return c.SendStatus(204)
		}
		return c.Next()
	})
	ctx := context.Background()

	client, err := firestore.NewClient(ctx, PROJECT_ID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	app.Get("/api/v1/incidents", GetIncidents(client))

	app.Get("/api/v1/incidents/:id", GetIncident(client))

	app.Post("/api/v1/incidents", CreateIncident(client))

	app.Patch("/api/v1/incidents/:id", UpdateIncident(client))

	app.Delete("/api/v1/incidents/:id", DeleteIncident(client))

	app.Use(func(c fiber.Ctx) error {
		return c.Status(404).JSON(fiber.Map{
			"error":         "Route not found in Cloud Run",
			"received_path": c.Path(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Fatal(app.Listen(":" + port))
}
