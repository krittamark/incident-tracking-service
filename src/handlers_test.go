package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v3"
)

// setupTestApp สร้าง Fiber App สำหรับการเทส
func setupTestApp(t *testing.T) (*fiber.App, *firestore.Client) {
	projectID := os.Getenv("GCLOUD_PROJECT_ID")
	if projectID == "" {
		projectID = "test-project"
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create firestore client: %v", err)
	}

	app := fiber.New()

	// ลงทะเบียนเส้นทางเหมือนใน main.go
	app.Get("/incidents", GetIncidents(client))
	app.Get("/incidents/:id", GetIncident(client))
	app.Post("/incidents", CreateIncident(client))
	app.Patch("/incidents/:id", UpdateIncident(client))
	app.Delete("/incidents/:id", DeleteIncident(client))

	return app, client
}

func TestIncidentHandlers(t *testing.T) {
	// ข้ามการเทสหากไม่ได้ตั้งค่า Emulator (เพื่อความปลอดภัย)
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("Skipping integration tests; FIRESTORE_EMULATOR_HOST not set")
	}

	app, _ := setupTestApp(t)
	var createdID string

	// 1. เทส POST /incidents (Success)
	t.Run("POST /incidents - Success", func(t *testing.T) {
		reqBody := CreateIncidentRequest{
			IncidentType:        "fire",
			IncidentDescription: "Test incident",
			ExactLocation:       "13.7563,100.5018",
			ImpactLevel:         2,
			Priority:            "Medium",
			ReportedBy:          "TestRunner",
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/incidents", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-IncidentTNX-Id", "test-trace-post")

		resp, _ := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var res CreateIncidentResponse
		json.NewDecoder(resp.Body).Decode(&res)
		createdID = res.IncidentID
		if createdID == "" {
			t.Error("Expected incidentID in response")
		}
	})

	// 2. เทส GET /incidents (Success)
	t.Run("GET /incidents - Success", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/incidents", nil)
		resp, _ := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// 3. เทส GET /incidents/:id (Success & Case Insensitivity)
	t.Run("GET /incidents/:id - Success", func(t *testing.T) {
		// ลองส่งเป็นตัวพิมพ์เล็ก ระบบต้องหาเจอเพราะเราแปลงเป็น Upper ใน Handler
		lowerID := strings.ToLower(createdID)
		req, _ := http.NewRequest("GET", "/incidents/"+lowerID, nil)
		resp, _ := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var incident Incident
		json.NewDecoder(resp.Body).Decode(&incident)
		if incident.IncidentID != createdID {
			t.Errorf("Expected ID %s, got %s", createdID, incident.IncidentID)
		}
	})

	// 4. เทส PATCH /incidents/:id (Success)
	t.Run("PATCH /incidents/:id - Success", func(t *testing.T) {
		reqBody := UpdateIncidentRequest{
			Status:    "DISPATCHED",
			UpdatedBy: "TestDispatcher",
			Detail:    "Units on the way",
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PATCH", "/incidents/"+createdID, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var updated Incident
		json.NewDecoder(resp.Body).Decode(&updated)
		if updated.Status != "DISPATCHED" {
			t.Errorf("Expected status DISPATCHED, got %s", updated.Status)
		}
		if len(updated.Timeline) < 2 {
			t.Error("Expected timeline to have at least 2 entries")
		}
	})

	// 5. เทส DELETE /incidents/:id (Success)
	t.Run("DELETE /incidents/:id - Success", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/incidents/"+createdID, nil)
		resp, _ := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// 6. เทส Error Response (404 Not Found & TraceID Uppercase)
	t.Run("GET /incidents/:id - Not Found", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/incidents/NON-EXISTENT-ID", nil)
		req.Header.Set("X-IncidentTNX-Id", "trace-err-123")

		resp, _ := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		var errRes ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errRes)
		if errRes.Error.Code != "NOT_FOUND" {
			t.Errorf("Expected error code NOT_FOUND, got %s", errRes.Error.Code)
		}
		// เช็คว่า TraceID ถูกแปลงเป็นตัวพิมพ์ใหญ่ตามที่ออกแบบไว้หรือไม่
		if errRes.Error.TraceID != "TRACE-ERR-123" {
			t.Errorf("Expected trace_id TRACE-ERR-123, got %s", errRes.Error.TraceID)
		}
	})
}
