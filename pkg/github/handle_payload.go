package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	eg "github.com/google/go-github/v66/github"
)

func HandlePayload(payload eg.WorkflowRunEvent) error {
	if payload.Action == nil {
		fmt.Printf("")
		return errors.New("Webhook Payload.Action was nil")
	}

	if *payload.Action == "completed" {
		fmt.Println("handle all the things, the workflow is %s!", *payload.Action)

		workflowId := payload.WorkflowRun.WorkflowID
		if workflowId != nil {
			panic(fmt.Errorf("Workflow.ID was nil"))
		}

		jobsUrl := payload.WorkflowRun.GetJobsURL()

		res, err := http.Get(jobsUrl)
		if err != nil {
			panic("failed to get jobs")
		}

		var foo eg.Jobs
		dec := json.NewDecoder(res.Body)
		err = dec.Decode(&foo)
		if err != nil {
			panic("couldn't decode")
		}
		jobs, err := json.MarshalIndent(foo, "", "  ")
		if err != nil {
			panic("marshal failed")
		}
		fmt.Println(string(jobs))

		// get pretty json string
		result, err := json.MarshalIndent(payload.Workflow, "", "  ")
		if err != nil {
			panic("marshal failed")
		}
		fmt.Println(string(result))
	} else {
		fmt.Printf("Skipping processing event: '%s'\n", *payload.Action)
	}

	return
}
