package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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

// AWS EventBridge Message Structure
type EventBridgeMessage struct {
	DetailType string `json:"detail-type"`
	Detail     struct {
		ReportRefID           string `json:"report_ref_id"`
		SuggestedIncidentData struct {
			Type          string `json:"type"`
			Description   string `json:"description"`
			SeverityLevel int    `json:"severity_level"`
			Location      struct {
				Lat interface{} `json:"lat"` // Use interface{} to handle string or float
				Lon interface{} `json:"lon"`
			} `json:"location"`
		} `json:"suggested_incident_data"`
		VerifiedBy        string `json:"verified_by"`
		VerificationNotes string `json:"verification_notes"`
	} `json:"detail"`
}

func HandleExternalReport(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	log.Printf("RECEIVED PAYLOAD: %s", string(body))

	// 1. Try AWS EventBridge Format (Based on user's actual payload)
	var ebMsg EventBridgeMessage
	if err := json.Unmarshal(body, &ebMsg); err == nil && ebMsg.DetailType != "" {
		log.Printf("Processing as EventBridge: %s", ebMsg.DetailType)
		
		// Convert Lat/Lon from interface{} to float64
		lat := parseCoordinate(ebMsg.Detail.SuggestedIncidentData.Location.Lat)
		lon := parseCoordinate(ebMsg.Detail.SuggestedIncidentData.Location.Lon)

		// Map to our standard event struct
		event := ExternalReportVerifiedEvent{
			ReportRefID: ebMsg.Detail.ReportRefID,
			VerifiedBy:  ebMsg.Detail.VerifiedBy,
			VerificationNotes: ebMsg.Detail.VerificationNotes,
		}
		event.SuggestedIncidentData.Type = ebMsg.Detail.SuggestedIncidentData.Type
		event.SuggestedIncidentData.Description = ebMsg.Detail.SuggestedIncidentData.Description
		event.SuggestedIncidentData.SeverityLevel = ebMsg.Detail.SuggestedIncidentData.SeverityLevel
		event.SuggestedIncidentData.Location.Lat = lat
		event.SuggestedIncidentData.Location.Lon = lon

		if err := createIncidentFromReport(r.Context(), event); err != nil {
			log.Printf("ERROR: EB Create incident failed: %v", err)
			http.Error(w, "Internal Error", 500)
			return
		}
		fmt.Fprint(w, "Success: EventBridge Processed")
		return
	}

	// 2. Try AWS SNS Format (Fallback)
	var snsMsg SNSMessage
	if err := json.Unmarshal(body, &snsMsg); err == nil && snsMsg.Type != "" {
		if snsMsg.Type == "SubscriptionConfirmation" {
			log.Printf("Confirming SNS Subscription: %s", snsMsg.SubscribeURL)
			http.Get(snsMsg.SubscribeURL)
			return
		}
		if snsMsg.Type == "Notification" {
			var event ExternalReportVerifiedEvent
			if err := json.Unmarshal([]byte(snsMsg.Message), &event); err == nil {
				if err := createIncidentFromReport(r.Context(), event); err != nil {
					log.Printf("ERROR: SNS Create failed: %v", err)
					return
				}
				fmt.Fprint(w, "Success: SNS Processed")
				return
			}
		}
	}

	// 3. Try Direct JSON (Fallback)
	var directEvent ExternalReportVerifiedEvent
	if err := json.Unmarshal(body, &directEvent); err == nil && directEvent.ReportRefID != "" {
		if err := createIncidentFromReport(r.Context(), directEvent); err != nil {
			log.Printf("ERROR: Direct Create failed: %v", err)
			return
		}
		fmt.Fprint(w, "Success: Direct Processed")
		return
	}

	log.Printf("WARNING: Payload did not match any format")
	fmt.Fprint(w, "Payload received but not processed")
}

func parseCoordinate(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
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
