package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/google/uuid"
)

var firestoreClient *firestore.Client

func init() {
	ctx := context.Background()
	projectID := os.Getenv("GCLOUD_PROJECT_ID")
	if projectID == "" {
		projectID = "cs366-incident-service"
	}
	var err error
	firestoreClient, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("firestore.NewClient: %v", err)
	}

	functions.HTTP("HandleExternalReport", HandleExternalReport)
}

// AWS SNS Message Structure
type SNSMessage struct {
	Type         string `json:"Type"`
	SubscribeURL string `json:"SubscribeURL"`
	Message      string `json:"Message"`
}

// ReportVerifiedEvent from External Service (Page 31)
type ExternalReportVerifiedEvent struct {
	ReportRefID           string `json:"report_ref_id"`
	SuggestedIncidentData struct {
		Type          string `json:"type"`
		Description   string `json:"description"`
		SeverityLevel int    `json:"severity_level"`
		Location      struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"location"`
	} `json:"suggested_incident_data"`
	VerifiedBy        string `json:"verified_by"`
	VerificationNotes string `json:"verification_notes"`
}

func HandleExternalReport(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle Preflight request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var snsMsg SNSMessage
	if err := json.Unmarshal(body, &snsMsg); err != nil {
		http.Error(w, "Invalid SNS format", http.StatusBadRequest)
		return
	}

	// 1. Handle SNS Subscription Confirmation
	if snsMsg.Type == "SubscriptionConfirmation" {
		log.Printf("Confirming SNS Subscription: %s", snsMsg.SubscribeURL)
		http.Get(snsMsg.SubscribeURL)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Handle Actual Notification
	if snsMsg.Type == "Notification" {
		var event ExternalReportVerifiedEvent
		// AWS SNS wraps the message as a string
		if err := json.Unmarshal([]byte(snsMsg.Message), &event); err != nil {
			log.Printf("Failed to unmarshal event message: %v", err)
			http.Error(w, "Invalid event message format", http.StatusBadRequest)
			return
		}

		// Create Incident from Verified Report
		if err := createIncidentFromReport(r.Context(), event); err != nil {
			log.Printf("Error creating incident: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Incident Created Successfully")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func createIncidentFromReport(ctx context.Context, event ExternalReportVerifiedEvent) error {
	incidentID := strings.ToUpper(uuid.New().String())
	now := time.Now().UTC()

	// Map severity 1-5 to 1-3
	impactLevel := event.SuggestedIncidentData.SeverityLevel / 2
	if impactLevel < 1 {
		impactLevel = 1
	} else if impactLevel > 3 {
		impactLevel = 3
	}

	// Build Incident Object
	incident := map[string]interface{}{
		"incident_id":          incidentID,
		"incident_type":        strings.ToLower(event.SuggestedIncidentData.Type),
		"incident_description": event.SuggestedIncidentData.Description,
		"exact_location":       fmt.Sprintf("%f,%f", event.SuggestedIncidentData.Location.Lat, event.SuggestedIncidentData.Location.Lon),
		"impact_level":         impactLevel,
		"priority":             "Medium", // Default
		"status":               "REPORTED",
		"reported_by":          "ReportVerify Service",
		"source_report_id":     event.ReportRefID,
		"created_at":           now,
		"updated_at":           now,
		"timeline": []map[string]interface{}{
			{
				"time":   now,
				"event":  "CREATED_FROM_REPORT",
				"detail": fmt.Sprintf("Verified by %s: %s", event.VerifiedBy, event.VerificationNotes),
			},
		},
	}

	_, err := firestoreClient.Collection("incidents").Doc(incidentID).Set(ctx, incident)
	if err != nil {
		return err
	}

	log.Printf("Successfully created incident %s from report %s", incidentID, event.ReportRefID)
	return nil
}
