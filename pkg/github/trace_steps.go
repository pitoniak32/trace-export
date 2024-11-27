package github

import (
	"context"
	"errors"
	"fmt"

	eg "github.com/google/go-github/v66/github"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TraceJobSteps(ctx context.Context, steps []*eg.TaskStep, tracer trace.Tracer) error {
	var stepErrors error = nil
	for _, step := range steps {
		err := TraceJobStep(ctx, step, tracer)
		if err != nil {
			stepErrors = errors.Join(stepErrors, err)
		}
	}

	return stepErrors
}

func TraceJobStep(ctx context.Context, step *eg.TaskStep, tracer trace.Tracer) error {
	stepName := step.GetName()
	if stepName == "" {
		stepName = "UNKNOWN"
	}

	stepNumber := step.GetNumber()
	if stepNumber == 0 {
		return &JobStepHandlingError{
			errMsg:   "Cannot find step number for the job step",
			stepName: &stepName,
		}
	}

	startTime := step.GetStartedAt().Time
	if startTime.IsZero() {
		return &JobStepHandlingError{
			errMsg:   "Cannot find 'run_start_time' on the job step",
			stepName: &stepName,
		}
	}

	// The time that this workflow run completed (since this is the completed handler)
	endTime := step.GetCompletedAt().Time
	if endTime.IsZero() {
		return &JobStepHandlingError{
			errMsg:   "Cannot find 'updated_at' on the job step",
			stepName: &stepName,
		}
	}

	attributes := []attribute.KeyValue{
		attribute.String("step.name", stepName),
		attribute.Int64("step.number", stepNumber),
	}

	// Start a new span using the workflow run tracer.
	_, span := tracer.Start(ctx, stepName, trace.WithTimestamp(startTime), trace.WithAttributes(attributes...))
	defer span.End(trace.WithTimestamp(endTime))

	return nil
}

type JobStepHandlingError struct {
	originErr error
	errMsg    string
	stepName  *string
}

func (w *JobStepHandlingError) Error() string {
	msg := "[FATAL]: An error occurred when handling workflow_job"
	if w.stepName != nil {
		msg = fmt.Sprintf("%s 'name = %s'", msg, *w.stepName)
	}
	if w.errMsg != "" {
		msg = fmt.Sprintf("%s: %s", msg, w.errMsg)
	}
	if w.originErr != nil {
		msg = fmt.Sprintf("%s: %s", msg, w.originErr)
	}
	return msg
}
