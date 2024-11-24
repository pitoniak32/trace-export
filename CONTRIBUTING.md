# Pre requisites

- install `just` (optional) - https://github.com/casey/just?tab=readme-ov-file#installation
- install `go` - https://go.dev/doc/install
- install `golangci-lint` - https://golangci-lint.run/welcome/install/#local-installation

# Notes for CI
- installing `golangci-lint` - https://golangci-lint.run/welcome/install/#ci-installation

# VSCode Setup Tips

1. Install `Go - Go Team at Google` extension
2. Update linter 
   1. Open `Settings`
   2. Search `go.lintTool`
   3. Set value to `golangci-lint` (make sure you have done the [Pre Reqs](#pre-requisites))

## Available Commands

This repo uses `just` (like make) to run common tasks.
- `just run`
- `just test`
- `just fmt`
- `just lint`

## Running Locally

You can forward webhooks from github to your localhost with the gh cli!
- https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/using-the-github-cli-to-forward-webhooks-for-testing

Example:
```bash
# make sure you have the extension installed
gh extension install cli/gh-webhook

# Start the docker containers and webhook forwarding
just forward
```

Once you have the forward running, just run the app
```bash
just run
```

And finally trigger a workflow on the repo that you are forwarding webhooks for.
