package api

import "encoding/json"

// Workflow mirrors models.WorkflowResponse on the orchard-ws side.
type Workflow struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Status       Status       `json:"status"`
	CreatedAt    OrchardTime  `json:"createdAt"`
	ActivatedAt  *OrchardTime `json:"activatedAt"`
	TerminatedAt *OrchardTime `json:"terminatedAt"`
}

// Activity mirrors models.ActivityResponse.
type Activity struct {
	WorkflowID   string          `json:"workflowId"`
	ActivityID   string          `json:"activityId"`
	Name         string          `json:"name"`
	ActivityType string          `json:"activityType"`
	ActivitySpec json.RawMessage `json:"activitySpec"`
	ResourceID   string          `json:"resourceId"`
	MaxAttempt   int             `json:"maxAttempt"`
	Status       Status          `json:"status"`
	CreatedAt    OrchardTime     `json:"createdAt"`
	ActivatedAt  *OrchardTime    `json:"activatedAt"`
	TerminatedAt *OrchardTime    `json:"terminatedAt"`
}

// ActivityAttempt is one execution attempt of an Activity.
type ActivityAttempt struct {
	WorkflowID              string          `json:"workflowId"`
	ActivityID              string          `json:"activityId"`
	Attempt                 int             `json:"attempt"`
	Status                  Status          `json:"status"`
	ErrorMessage            string          `json:"errorMessage"`
	ResourceID              string          `json:"resourceId"`
	ResourceInstanceAttempt int             `json:"resourceInstanceAttempt"`
	AttemptSpec             json.RawMessage `json:"attemptSpec"`
	CreatedAt               OrchardTime     `json:"createdAt"`
	ActivatedAt             *OrchardTime    `json:"activatedAt"`
	TerminatedAt            *OrchardTime    `json:"terminatedAt"`
}

// Resource mirrors models.ResourceResponse. terminateAfter is hours.
type Resource struct {
	WorkflowID     string          `json:"workflowId"`
	ResourceID     string          `json:"resourceId"`
	Name           string          `json:"name"`
	ResourceType   string          `json:"resourceType"`
	ResourceSpec   json.RawMessage `json:"resourceSpec"`
	MaxAttempt     int             `json:"maxAttempt"`
	Status         Status          `json:"status"`
	CreatedAt      OrchardTime     `json:"createdAt"`
	ActivatedAt    *OrchardTime    `json:"activatedAt"`
	TerminatedAt   *OrchardTime    `json:"terminatedAt"`
	TerminateAfter float64         `json:"terminateAfter"`
}

// ResourceInstance is one instance of a Resource.
type ResourceInstance struct {
	WorkflowID      string          `json:"workflowId"`
	ResourceID      string          `json:"resourceId"`
	InstanceAttempt int             `json:"instanceAttempt"`
	InstanceSpec    json.RawMessage `json:"instanceSpec"`
	Status          Status          `json:"status"`
	ErrorMessage    string          `json:"errorMessage"`
	CreatedAt       OrchardTime     `json:"createdAt"`
	ActivatedAt     *OrchardTime    `json:"activatedAt"`
	TerminatedAt    *OrchardTime    `json:"terminatedAt"`
}

// ActivitiesResponse is the body of GET /v1/workflow/{id}/activities.
type ActivitiesResponse struct {
	Workflow   Workflow   `json:"workflow"`
	Activities []Activity `json:"activities"`
}

// ActivityAttemptsResponse is the body of GET /v1/workflow/{id}/activity/{actId}.
type ActivityAttemptsResponse struct {
	Activity Activity          `json:"activity"`
	Attempts []ActivityAttempt `json:"attempts"`
}

// ResourcesResponse is the body of GET /v1/workflow/{id}/resources.
type ResourcesResponse struct {
	Workflow  Workflow   `json:"workflow"`
	Resources []Resource `json:"resources"`
}

// ResourceInstancesResponse is the body of GET /v1/workflow/{id}/resource/{rscId}.
type ResourceInstancesResponse struct {
	Resource  Resource           `json:"resource"`
	Instances []ResourceInstance `json:"instances"`
}

// StatusCounts maps each status name to a workflow count.
// Unknown keys are silently retained so a future status doesn't break us.
type StatusCounts map[Status]int

// Get returns the count for s, or zero if not present.
func (c StatusCounts) Get(s Status) int { return c[s] }

// DailyCount is one row from GET /v1/stats/daily.
type DailyCount struct {
	Date   OrchardDate `json:"date"`
	Status Status      `json:"status"`
	Count  int         `json:"count"`
}

// PatternCount is one row from GET /v1/stats/pattern.
type PatternCount struct {
	DayOfWeek int `json:"dow"`  // 0=Sunday … 6=Saturday (matches Postgres EXTRACT(DOW))
	Hour      int `json:"hour"` // 0..23
	Count     int `json:"count"`
}
