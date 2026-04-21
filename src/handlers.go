package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

// UpdateIncident returns a handler that updates an incident status and timeline in Firestore.
func UpdateIncident(client *firestore.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := context.Background()
		id := strings.ToUpper(c.Params("id"))
		traceID := strings.ToUpper(c.Get("X-IncidentTNX-Id"))

		var req UpdateIncidentRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    "BAD_REQUEST",
					Message: "Invalid request body",
					Details: []string{err.Error()},
					TraceID: traceID,
				},
			})
		}

		docRef := client.Collection("incidents").Doc(id)
		var updatedIncident Incident

		err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			doc, err := tx.Get(docRef)
			if err != nil {
				return err
			}

			var incident Incident
			if err := doc.DataTo(&incident); err != nil {
				return err
			}

			now := time.Now().UTC()
			incident.Status = req.Status
			incident.UpdatedAt = now
			incident.Timeline = append(incident.Timeline, TimelineEntry{
				Time:   now,
				Event:  "STATUS_CHANGED",
				Detail: fmt.Sprintf("Updated to %s by %s. %s", req.Status, req.UpdatedBy, req.Detail),
			})

			updatedIncident = incident
			return tx.Set(docRef, incident)
		})

		if err != nil {
			status := fiber.StatusInternalServerError
			code := "INTERNAL_SERVER_ERROR"
			message := "Failed to update incident"

			if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
				status = fiber.StatusNotFound
				code = "NOT_FOUND"
				message = "Incident not found"
			}

			return c.Status(status).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    code,
					Message: message,
					Details: []string{err.Error()},
					TraceID: traceID,
				},
			})
		}

		return c.JSON(updatedIncident)
	}
}

// DeleteIncident returns a handler that deletes an incident from Firestore.
func DeleteIncident(client *firestore.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := context.Background()
		id := strings.ToUpper(c.Params("id"))
		traceID := strings.ToUpper(c.Get("X-IncidentTNX-Id"))

		_, err := client.Collection("incidents").Doc(id).Delete(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Failed to delete incident",
					Details: []string{err.Error()},
					TraceID: traceID,
				},
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Incident deleted successfully",
		})
	}
}

// GetIncident returns a handler that fetches a single incident by ID from Firestore.

func CreateIncident(client *firestore.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := context.Background()
		var req CreateIncidentRequest
		traceID := strings.ToUpper(c.Get("X-IncidentTNX-Id"))

		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    "BAD_REQUEST",
					Message: "Invalid request body",
					Details: []string{err.Error()},
					TraceID: traceID,
				},
			})
		}

		incidentID := strings.ToUpper(uuid.New().String())
		now := time.Now().UTC()

		incident := Incident{
			IncidentID:               incidentID,
			IncidentType:             req.IncidentType,
			IncidentDescription:      req.IncidentDescription,
			ExactLocation:            req.ExactLocation,
			ExactLocationDescription: req.ExactLocationDescription,
			ImpactLevel:              req.ImpactLevel,
			Priority:                 req.Priority,
			Status:                   "REPORTED",
			ReportedBy:               req.ReportedBy,
			CreatedAt:                now,
			UpdatedAt:                now,
			Timeline: []TimelineEntry{
				{
					Time:   now,
					Event:  "Created",
					Detail: "Incident verified",
				},
			},
		}

		_, err := client.Collection("incidents").Doc(incidentID).Set(ctx, incident)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Failed to save incident to database",
					Details: []string{err.Error()},
					TraceID: traceID,
				},
			})
		}

		return c.Status(fiber.StatusCreated).JSON(CreateIncidentResponse{
			IncidentID: incidentID,
			Status:     incident.Status,
			CreatedAt:  incident.CreatedAt,
			Message:    "Incident created successfully",
		})
	}
}

// GetIncidents returns a handler that fetches all incidents from Firestore.
func GetIncidents(client *firestore.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := context.Background()
		var incidents []Incident
		traceID := strings.ToUpper(c.Get("X-IncidentTNX-Id"))

		// Assuming the collection name is "incidents"
		iter := client.Collection("incidents").Documents(ctx)
		defer iter.Stop()

		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
					Error: ErrorDetail{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "Failed to fetch incidents from database",
						TraceID: traceID,
					},
				})
			}

			var incident Incident
			if err := doc.DataTo(&incident); err != nil {
				// Skip documents that can't be parsed into the Incident struct
				continue
			}
			incidents = append(incidents, incident)
		}

		// Return empty slice instead of null if no incidents are found
		if incidents == nil {
			incidents = []Incident{}
		}

		return c.JSON(incidents)
	}
}

// GetIncident returns a handler that fetches a single incident by ID from Firestore.
func GetIncident(client *firestore.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := context.Background()
		id := strings.ToUpper(c.Params("id"))
		traceID := strings.ToUpper(c.Get("X-IncidentTNX-Id"))

		doc, err := client.Collection("incidents").Doc(id).Get(ctx)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    "NOT_FOUND",
					Message: "Incident not found",
					Details: []string{err.Error()},
					TraceID: traceID,
				},
			})
		}

		var incident Incident
		if err := doc.DataTo(&incident); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    "INTERNAL_SERVER_ERROR",
					Message: "Failed to parse incident data",
					TraceID: traceID,
				},
			})
		}

		return c.JSON(incident)
	}
}
