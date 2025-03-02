package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	eg "github.com/google/go-github/v66/github"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func HandlePayload(payload eg.WorkflowRunEvent, tracer trace.Tracer) error {
	payloadAction := payload.GetAction()
	if payloadAction == "" {
		return errors.New("Webhook Payload.Action was not found.")
	}
	workflowRun := payload.GetWorkflowRun()
	if workflowRun == nil {
		return errors.New("Expecting a workflow_run for all webhook events.")
	}
	workflowRunID := workflowRun.GetID()
	if workflowRunID == 0 {
		return errors.New("Expecting a workflow_run.id for all webhook events.")
	}

	switch payloadAction {
	case "requested":
		return HandleWorkflowRunRequested(*workflowRun, workflowRunID)
	case "in_progress":
		return HandleWorkflowRunInProgress(*workflowRun, workflowRunID)
	case "completed":
		return HandleWorkflowRunCompleted(*workflowRun, workflowRunID, tracer)
	default:
		return HandleWorkflowRunUnknown(*workflowRun, workflowRunID)
	}
}

func HandleWorkflowRunRequested(_w eg.WorkflowRun, runId int64) error {
	slog.Debug("skipping workflow run", "run.id", runId, "run.status", "requested")
	return nil
}

func HandleWorkflowRunInProgress(_w eg.WorkflowRun, runId int64) error {
	slog.Debug("skipping workflow run", "run.id", runId, "run.status", "in_progress")
	return nil
}

// func AttributesFromCustomProperties(props []*eg.CustomPropertyValue) []attribute.KeyValue {
// 	attributes := []attribute.KeyValue{
// 		attribute.Int64("workflow_run.id", ),
// 	}
// 	return attributes
// }

func HandleWorkflowRunCompleted(w eg.WorkflowRun, runId int64, tracer trace.Tracer) error {
	// client := github.NewClient(nil).WithAuthToken("")
	// props, res, err := client.Repositories.GetAllCustomPropertyValues(context.Background(), "", "")
	// if err != nil {
	// 	panic("failed to get repo custom properties")
	// }

	startTime := w.GetRunStartedAt().Time
	if startTime.IsZero() {
		return &WorkflowRunHandlingError{
			errMsg:        "Cannot find 'run_start_time' on the workflow_run event",
			workflowRunID: &runId,
		}
	}

	// The time that this workflow run completed (since this is the completed handler)
	endTime := w.GetUpdatedAt().Time
	if endTime.IsZero() {
		return &WorkflowRunHandlingError{
			errMsg:        "Cannot find 'updated_at' on the workflow_run event",
			workflowRunID: &runId,
		}
	}

	spanName := w.GetName()
	if spanName == "" {
		spanName = "UNKNOWN"
	}

	attributes := []attribute.KeyValue{
		attribute.Int64("workflow_run.id", runId),
	}

	// Start a new span using the workflow run tracer.
	ctx, span := tracer.Start(context.Background(), spanName, trace.WithTimestamp(startTime), trace.WithAttributes(attributes...))
	defer span.End(trace.WithTimestamp(endTime))

	slog.Debug("handling workflow run", "run.id", runId, "run.status", "completed")

	jobsUrl := w.GetJobsURL()
	if jobsUrl == "" {
		return &WorkflowRunHandlingError{
			errMsg:        "Cannot find 'jobs_url' on the workflow event",
			workflowRunID: &runId,
		}
	}

	res, err := http.Get(jobsUrl)
	if err != nil {
		return &WorkflowRunHandlingError{
			originErr:     err,
			errMsg:        fmt.Sprintf("Request to '%s' to fetch jobs failed", jobsUrl),
			workflowRunID: &runId,
		}
	}

	var jobs eg.Jobs
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&jobs)
	if err != nil {
		panic("couldn't decode")
	}

	err = TraceWorkflowJobs(ctx, startTime, jobs, tracer)
	if err != nil {
		return err
	}

	return nil
}

func HandleWorkflowRunUnknown(w eg.WorkflowRun, runId int64) error {
	// TODO: we need to add this to the trace for the webhook request to know if github is sending bad webhook actions
	return &WorkflowRunHandlingError{
		errMsg:        fmt.Sprintf("Workflow run 'id = %d' action is 'unknown'... There is an issue with the payloads being received from GitHub.\n", runId),
		workflowRunID: &runId,
	}
}

type WorkflowRunHandlingError struct {
	originErr     error
	errMsg        string
	workflowRunID *int64
}

func (w *WorkflowRunHandlingError) Error() string {
	msg := "[FATAL]: An error occurred when handling workflow_run"
	if w.workflowRunID != nil {
		msg = fmt.Sprintf("%s 'id = %d'", msg, *w.workflowRunID)
	}
	if w.errMsg != "" {
		msg = fmt.Sprintf("%s: %s", msg, w.errMsg)
	}
	if w.originErr != nil {
		msg = fmt.Sprintf("%s: %s", msg, w.originErr)
	}
	return msg
}
