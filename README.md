# trace-export

##  Export traces for GitHub Actions!

This project is meant to export traces of GitHub Action workflow runs. It will create a trace with a root Span that captures the full workflow run. The jobs, and steps of the workflow will be child spans of the workflow run trace. This gives key information about the time it took for each job, step, and the full workflow to complete, and reflect their respective status.

### Benefits

Once these traces are generated you can go anything you would like with them. In your collector you could generate metric points to create reports on high level statistics of your workflow runs. Or if you have a tracing backend that supports it, you could create those reports directly with the traces!

- You now have the power to direct your workflow run data to any backend supported by Opentelemetry!
- You have access to very rich workflow run data that can be filtered or trimmed as needed.
- The data is not locked to a specific vendors tooling! (See [Vendors](https://opentelemetry.io/ecosystem/vendors/) who natively support OpenTelemetry!)

### How
The way this is done is by taking a [workflow_run](https://docs.github.com/en/webhooks/webhook-events-and-payloads#workflow_run) webhook from github and reacting to `completed` events. When an event with the `completed` action is recieved it will be handled by the service. Downloading all the associated jobs of the workflow run, and generating spans. This workflow run trace will be exported to the configured otlp tracing backend that is configured by your app (typically a https://opentelemetry.io/docs/collector/) but in some cases it might make more sense to directly export to a specific backend.


### TODO
- [ ] Try out the testing with traces approach - https://opentelemetry.io/blog/2023/testing-otel-demo/
- [ ] Handle creating traces for all jobs and steps.
- [ ] Add tests for all handle logic.
- [ ] Make this usable as an action, and a standalone service!
  - [ ] Demo of using this service as a standalone github action job
- [ ] Demo of using this service in a cloud function

