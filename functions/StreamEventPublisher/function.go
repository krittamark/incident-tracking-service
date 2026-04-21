package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"github.com/googleapis/google-cloudevents-go/cloud/firestoredata"
)

var (
	pubsubClient *pubsub.Client
	topic        *pubsub.Topic
	once         sync.Once
)

func init() {
	functions.CloudEvent("ProcessIncidentEvent", ProcessIncidentEvent)
}

func setup() {
	once.Do(func() {
		ctx := context.Background()
		projectID := "cs366-incident-service"
		var err error
		pubsubClient, err = pubsub.NewClient(ctx, projectID)
		if err != nil {
			log.Fatalf("pubsub.NewClient: %v", err)
		}
		topic = pubsubClient.Topic("incident-events")
	})
}

type IncidentEvent struct {
	EventID   string      `json:"eventId"`
	EventType string      `json:"eventType"`
	SentAt    time.Time   `json:"sentAt"`
	Data      interface{} `json:"data"`
}

func ProcessIncidentEvent(ctx context.Context, e event.Event) error {
	setup()
	
	var data firestoredata.DocumentEventData
	// WORKAROUND: Use proto.Unmarshal for Binary Protobuf data from Eventarc Firestore trigger
	if err := proto.Unmarshal(e.Data(), &data); err != nil {
		return fmt.Errorf("proto.Unmarshal (Binary): %v", err)
	}

	if data.OldValue == nil && data.Value != nil {
		return handleCreate(ctx, data.Value)
	} else if data.OldValue != nil && data.Value != nil {
		return handleUpdate(ctx, data.OldValue, data.Value)
	}
	return nil
}

func handleCreate(ctx context.Context, doc *firestoredata.Document) error {
	fields := doc.Fields
	createdData := map[string]interface{}{
		"incident_id":          fields["incident_id"].GetStringValue(),
		"incident_type":        fields["incident_type"].GetStringValue(),
		"incident_description": fields["incident_description"].GetStringValue(),
		"exact_location":       fields["exact_location"].GetStringValue(),
		"impact_level":         fields["impact_level"].GetIntegerValue(),
		"priority":             fields["priority"].GetStringValue(),
		"status":               fields["status"].GetStringValue(),
		"reported_by":          fields["reported_by"].GetStringValue(),
		"source_report_id":     fields["source_report_id"].GetStringValue(),
		"created_at":           time.Now().UTC(),
	}

	evt := IncidentEvent{
		EventID:   uuid.New().String(),
		EventType: "INCIDENT_CREATED",
		SentAt:    time.Now().UTC(),
		Data:      createdData,
	}
	return publishToPubSub(ctx, evt)
}

func handleUpdate(ctx context.Context, oldDoc, newDoc *firestoredata.Document) error {
	oldStatus := oldDoc.Fields["status"].GetStringValue()
	newStatus := newDoc.Fields["status"].GetStringValue()

	if oldStatus == newStatus {
		return nil
	}

	detail := ""
	if timeline := newDoc.Fields["timeline"].GetArrayValue(); timeline != nil {
		vals := timeline.Values
		if len(vals) > 0 {
			lastEntry := vals[len(vals)-1].GetMapValue()
			if lastEntry != nil {
				detail = lastEntry.Fields["detail"].GetStringValue()
			}
		}
	}

	evt := IncidentEvent{
		EventID:   uuid.New().String(),
		EventType: "STATUS_CHANGED",
		SentAt:    time.Now().UTC(),
		Data: map[string]interface{}{
			"incident_id": newDoc.Fields["incident_id"].GetStringValue(),
			"old_status":  oldStatus,
			"new_status":  newStatus,
			"updated_by":  "Incident Service",
			"detail":      detail,
		},
	}
	return publishToPubSub(ctx, evt)
}

func publishToPubSub(ctx context.Context, payload IncidentEvent) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %v", err)
	}

	result := topic.Publish(ctx, &pubsub.Message{
		Data: data,
	})
	_, err = result.Get(ctx)
	if err != nil {
		return fmt.Errorf("topic.Publish.Get: %v", err)
	}
	log.Printf("Successfully published %s event", payload.EventType)
	return nil
}
