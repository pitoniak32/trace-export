package github

import (
	"context"
	"errors"
	"fmt"
	"time"

	eg "github.com/google/go-github/v66/github"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TraceWorkflowJobs(ctx context.Context, workflowStart time.Time, jobs eg.Jobs, tracer trace.Tracer) error {
	if *jobs.TotalCount < 1 {
		return errors.New("not enough jobs in workflow to trace")
	}

	firstJobStartTime := jobs.Jobs[0].GetStartedAt().Time
	if firstJobStartTime.IsZero() {
		return errors.New("first job did not have a start time")
	}
	// track how long the first job was queued before it was picked up by a runner.
	ctx, span := tracer.Start(ctx, "Queued", trace.WithTimestamp(workflowStart))
	span.End(trace.WithTimestamp(firstJobStartTime))

	var jobErrors error = nil
	for _, job := range jobs.Jobs {
		err := TraceWorkflowJob(ctx, job, tracer)
		if err != nil {
			jobErrors = errors.Join(jobErrors, err)
		}
	}

	return jobErrors
}

func TraceWorkflowJob(ctx context.Context, job *eg.WorkflowJob, tracer trace.Tracer) error {
	jobId := job.GetID()
	if jobId == 0 {
		return errors.New("Expecting a workflow_job.id for all workflow_jobs.")
	}

	startTime := job.GetStartedAt().Time
	if startTime.IsZero() {
		return &WorkflowJobHandlingError{
			errMsg:        "Cannot find 'run_start_time' on the workflow_job",
			workflowJobID: &jobId,
		}
	}

	// The time that this workflow run completed (since this is the completed handler)
	endTime := job.GetCompletedAt().Time
	if endTime.IsZero() {
		return &WorkflowJobHandlingError{
			errMsg:        "Cannot find 'updated_at' on the workflow_job",
			workflowJobID: &jobId,
		}
	}

	jobSpanName := job.GetName()
	if jobSpanName == "" {
		jobSpanName = "UNKNOWN"
	}

	attributes := []attribute.KeyValue{
		attribute.Int64("workflow_job.id", jobId),
	}

	// Start a new span using the workflow run tracer.
	ctx, span := tracer.Start(ctx, jobSpanName, trace.WithTimestamp(startTime), trace.WithAttributes(attributes...))
	defer span.End(trace.WithTimestamp(endTime))

	err := TraceJobSteps(ctx, job.Steps, tracer)
	if err != nil {
		return err
	}

	return nil
}

type WorkflowJobHandlingError struct {
	originErr     error
	errMsg        string
	workflowJobID *int64
}

func (w *WorkflowJobHandlingError) Error() string {
	msg := "[FATAL]: An error occurred when handling workflow_job"
	if w.workflowJobID != nil {
		msg = fmt.Sprintf("%s 'id = %d'", msg, *w.workflowJobID)
	}
	if w.errMsg != "" {
		msg = fmt.Sprintf("%s: %s", msg, w.errMsg)
	}
	if w.originErr != nil {
		msg = fmt.Sprintf("%s: %s", msg, w.originErr)
	}
	return msg
}
