package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	eg "github.com/google/go-github/v66/github"
)

func HandlePayload(payload eg.WorkflowRunEvent) error {
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
		return HandleWorkflowRunCompleted(*workflowRun, workflowRunID)
	default:
		return HandleWorkflowRunUnknown(*workflowRun, workflowRunID)
	}
}

func HandleWorkflowRunRequested(_w eg.WorkflowRun, runId int64) error {
	fmt.Printf("[SKIP]: workflow run '%d' is 'requested'.\n", runId)
	return nil
}

func HandleWorkflowRunInProgress(_w eg.WorkflowRun, runId int64) error {
	fmt.Printf("[SKIP]: workflow run '%d' is 'in_progress'.\n", runId)
	return nil
}

func HandleWorkflowRunCompleted(w eg.WorkflowRun, runId int64) error {
	fmt.Printf("[HANDLE]: workflow run '%d' is 'completed'.\n", runId)

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

	var foo eg.Jobs
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&foo)
	if err != nil {
		panic("couldn't decode")
	}
	fmt.Printf("Found %d job(s) to trace!\n", foo.GetTotalCount())

	// jobs, err := json.MarshalIndent(foo, "", "  ")
	// if err != nil {
	// 	panic("marshal failed")
	// }
	// fmt.Println(string(jobs))

	// get pretty json string
	// result, err := json.MarshalIndent(w, "", "  ")
	// if err != nil {
	// 	panic("marshal failed")
	// }
	// fmt.Println(string(result))
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
