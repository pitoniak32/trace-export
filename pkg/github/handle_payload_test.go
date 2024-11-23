package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eg "github.com/google/go-github/v66/github"
)

func TestHandlePayloadRequestedOrInProgress(t *testing.T) {
	var workflowRun eg.WorkflowRun
	result := HandleWorkflowRunRequested(workflowRun, 0)

	if result != nil {
		t.Errorf("[requested]: got %s, want nil", result)
	}

	result = HandleWorkflowRunInProgress(workflowRun, 0)

	if result != nil {
		t.Errorf("[in_progress] got %s, want nil", result)
	}
}

func TestHandlePayloadCompleted(t *testing.T) {

	// Setup a test http server that will be called as workflow_run.job_url
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"total_count": 0, "jobs": []}`))
		if err != nil {
			t.Fatalf("writing response from jobs mock failed.")
		}
	}))
	defer server.Close()

	var tests = []struct {
		jobsUrl *string
		want    string
	}{
		{&server.URL, ""},
		{nil, "An error occurred when handling workflow_run 'id = 1234': Cannot find 'jobs_url' on the workflow event"},
	}

	for _, tt := range tests {
		testName := "nil"
		if tt.jobsUrl != nil {
			testName = fmt.Sprintf("job_url: %s", *tt.jobsUrl)
		}
		t.Run(testName, func(t *testing.T) {

			jobsUrl := tt.jobsUrl
			var workflowRun eg.WorkflowRun = eg.WorkflowRun{
				JobsURL: jobsUrl,
			}

			err := HandleWorkflowRunCompleted(workflowRun, 1234)

			if err != nil {
				if !strings.Contains(err.Error(), tt.want) {
					t.Errorf("got %s, want %s", err.Error(), tt.want)
				}
			}

		})
	}

}

func TestHandlePayloadUnknown(t *testing.T) {
	expected := "action is 'unknown'"

	var workflowRun eg.WorkflowRun
	result := HandleWorkflowRunUnknown(workflowRun, 0)

	if !strings.Contains(result.Error(), expected) {
		t.Errorf("got '%s', want error containing '%s'", result, expected)
	}
}
