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

	functions.HTTP("HandleMissionStatusChange", HandleMissionStatusChange)
}

// SNSMessage represents the AWS SNS message structure
type SNSMessage struct {
	Type         string `json:"Type"`
	SubscribeURL string `json:"SubscribeURL"`
	Message      string `json:"Message"`
}

type MissionStatusChangedEvent struct {
	SchemaVersion string `json:"schema_version"`
	MissionID     string `json:"mission_id"`
	RequestID     string `json:"requestId"`
	IncidentID    string `json:"incident_id"`
	RescueTeamID  string `json:"rescue_team_id"`
	OldStatus     string `json:"old_status"`
	NewStatus     string `json:"new_status"`
	ChangedAt     string `json:"changed_at"`
	ChangedBy     string `json:"changed_by"`
}

func HandleMissionStatusChange(w http.ResponseWriter, r *http.Request) {
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

	// 1. Try SNS Format (Common for cross-cloud subscription)
	var snsMsg SNSMessage
	if err := json.Unmarshal(body, &snsMsg); err == nil && snsMsg.Type != "" {
		if snsMsg.Type == "SubscriptionConfirmation" {
			log.Printf("Confirming SNS Subscription: %s", snsMsg.SubscribeURL)
			resp, err := http.Get(snsMsg.SubscribeURL)
			if err != nil {
				log.Printf("ERROR: Failed to confirm subscription: %v", err)
				http.Error(w, "Failed to confirm subscription", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()
			fmt.Fprint(w, "Subscription confirmed")
			return
		}
		
		if snsMsg.Type == "Notification" {
			var event MissionStatusChangedEvent
			if err := json.Unmarshal([]byte(snsMsg.Message), &event); err == nil {
				if err := processEvent(r.Context(), event, w); err != nil {
					return
				}
				return
			}
		}
	}

	// 2. Try Direct JSON
	var event MissionStatusChangedEvent
	if err := json.Unmarshal(body, &event); err == nil && event.IncidentID != "" {
		processEvent(r.Context(), event, w)
		return
	}

	log.Printf("WARNING: Payload did not match any expected format")
	http.Error(w, "Payload format not recognized", http.StatusBadRequest)
}

func processEvent(ctx context.Context, event MissionStatusChangedEvent, w http.ResponseWriter) error {
	if event.IncidentID == "" {
		log.Printf("ERROR: IncidentID is missing in payload")
		http.Error(w, "IncidentID is missing", http.StatusBadRequest)
		return fmt.Errorf("incident id missing")
	}

	log.Printf("Processing mission status change for incident %s: %s -> %s", event.IncidentID, event.OldStatus, event.NewStatus)

	if err := updateIncident(ctx, event); err != nil {
		log.Printf("ERROR: Failed to update incident %s: %v", event.IncidentID, err)
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			http.Error(w, "Incident not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return err
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Incident status updated successfully")
	return nil
}

func updateIncident(ctx context.Context, event MissionStatusChangedEvent) error {
	docRef := firestoreClient.Collection("incidents").Doc(strings.ToUpper(event.IncidentID))

	return firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil {
			return err
		}

		if !doc.Exists() {
			return fmt.Errorf("incident %s not found", event.IncidentID)
		}

		now := time.Now().UTC()
		
		// Map the mission status to incident status if needed. 
		// For now, we'll assume the mission's new_status is what we want for the incident status 
		// or at least we update the incident to reflect this change.
		
		timelineEntry := map[string]interface{}{
			"time":   now,
			"event":  "MISSION_STATUS_CHANGED",
			"detail": fmt.Sprintf("Mission %s changed from %s to %s by %s. Team: %s", 
				event.MissionID, event.OldStatus, event.NewStatus, event.ChangedBy, event.RescueTeamID),
		}

		updates := []firestore.Update{
			{Path: "status", Value: event.NewStatus},
			{Path: "updated_at", Value: now},
			{Path: "timeline", Value: firestore.ArrayUnion(timelineEntry)},
		}

		return tx.Update(docRef, updates)
	})
}
