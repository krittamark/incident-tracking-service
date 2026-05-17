package main

import "time"

// ─────────────────────────────────────────────
// Shared / Common
// ─────────────────────────────────────────────

// TimelineEntry records the action history at each stage of an incident.
type TimelineEntry struct {
	Time   time.Time `json:"time" firestore:"time"`
	Event  string    `json:"event" firestore:"event"`
	Detail string    `json:"detail" firestore:"detail"`
}

// ErrorDetail represents the standard error response structure.
type ErrorDetail struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
	TraceID string   `json:"trace_id"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ─────────────────────────────────────────────
// API #1 – POST /incidents (Create Incident)
// ─────────────────────────────────────────────

type CreateIncidentRequest struct {
	IncidentType             string `json:"incident_type"`
	IncidentDescription      string `json:"incident_description"`
	ExactLocation            string `json:"exact_location"`
	ExactLocationDescription string `json:"exact_location_description"`
	ImpactLevel              int    `json:"impact_level"`
	Priority                 string `json:"priority"`
	ReportedBy               string `json:"reported_by"`
	SourceReportID           string `json:"source_report_id"`
}

type CreateIncidentResponse struct {
	IncidentID string    `json:"incident_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	Message    string    `json:"message"`
}

// ─────────────────────────────────────────────
// API #2 – GET /incidents/{id}
// API #4 – GET /incidents/
// ─────────────────────────────────────────────

// Incident holds the master data for a single incident.
type Incident struct {
	IncidentID               string          `json:"incident_id" firestore:"incident_id"`
	IncidentType             string          `json:"incident_type" firestore:"incident_type"`
	IncidentDescription      string          `json:"incident_description" firestore:"incident_description"`
	ExactLocation            string          `json:"exact_location" firestore:"exact_location"`
	ExactLocationDescription string          `json:"exact_location_description,omitempty" firestore:"exact_location_description,omitempty"`
	ImpactLevel              int             `json:"impact_level" firestore:"impact_level"`
	Priority                 string          `json:"priority" firestore:"priority"`
	Status                   string          `json:"status" firestore:"status"`
	ReportedBy               string          `json:"reported_by" firestore:"reported_by"`
	SourceReportID           string          `json:"source_report_id,omitempty" firestore:"source_report_id,omitempty"`
	TransactionID            string          `json:"-" firestore:"transaction_id,omitempty"`
	CreatedAt                time.Time       `json:"created_at" firestore:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at" firestore:"updated_at"`
	Timeline                 []TimelineEntry `json:"timeline" firestore:"timeline"`
}

// ─────────────────────────────────────────────
// API #3 – PATCH /incidents/{id}
// ─────────────────────────────────────────────

type UpdateIncidentRequest struct {
	Status    string `json:"status"`
	UpdatedBy string `json:"updated_by"`
	Detail    string `json:"detail"`
}

// The PATCH response reuses the Incident struct above.

// ─────────────────────────────────────────────
// Async Message #1 – INCIDENT_CREATED
// ─────────────────────────────────────────────

type IncidentCreatedData struct {
	IncidentID               string    `json:"incident_id"`
	IncidentType             string    `json:"incident_type"`
	IncidentDescription      string    `json:"incident_description"`
	ExactLocation            string    `json:"exact_location"`
	ExactLocationDescription string    `json:"exact_location_description,omitempty"`
	ImpactLevel              int       `json:"impact_level"`
	Priority                 string    `json:"priority"`
	Status                   string    `json:"status"`
	ReportedBy               string    `json:"reported_by"`
	SourceReportID           string    `json:"source_report_id,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
}

type IncidentCreatedEvent struct {
	EventID   string              `json:"eventId"`
	EventType string              `json:"eventType"` // "INCIDENT_CREATED"
	SentAt    time.Time           `json:"sentAt"`
	Data      IncidentCreatedData `json:"data"`
}

// ─────────────────────────────────────────────
// Async Message #2 – STATUS_CHANGED
// ─────────────────────────────────────────────

type StatusChangedData struct {
	IncidentID string `json:"incident_id"`
	OldStatus  string `json:"old_status"`
	NewStatus  string `json:"new_status"`
	UpdatedBy  string `json:"updated_by"`
	Detail     string `json:"detail,omitempty"`
}

type StatusChangedEvent struct {
	EventID   string            `json:"eventId"`
	EventType string            `json:"eventType"` // "STATUS_CHANGED"
	SentAt    time.Time         `json:"sentAt"`
	Data      StatusChangedData `json:"data"`
}

// ─────────────────────────────────────────────
// Async Message #3 – MISSION_STATUS_CHANGED
// ─────────────────────────────────────────────

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
