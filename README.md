# trace-export

Export traces for github actions in go



https://opentelemetry.io/blog/2023/testing-otel-demo/

# Usage

This repo uses `just` (like make) to run common tasks.
- `just run`
- `just test`
- `just fmt`
- `just lint`

# Forward Repo Webhooks

- https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/using-the-github-cli-to-forward-webhooks-for-testing

```bash
gh webhook forward --repo=pitoniak32/trace-export --events=workflow_run --url=http://localhost:8080/webhook
```