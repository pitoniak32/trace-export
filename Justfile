# This will ensure `.env` file is evaluated before recipes are run!
set dotenv-load

# NOTE: these will be executed from project root!
# for a sanity check try `just pwd` from a non root dir.

# Run the entry point
run:
  go run main.go

# Test all packages
test flags="": 
  go test {{flags}} ./...

# Lint all packages
lint: fmt
  golangci-lint run

# Format all packages
fmt:
  go fmt ./...

# Setup forward for github workflow_run webhooks to localhost for local dev
forward: local-up
  gh webhook forward --repo=pitoniak32/trace-export --events=workflow_run --url=http://localhost:8080/webhook

# Setup supporting services for local dev
local-up:
  @echo "Starting Docker Services..."
  docker compose -f ./docker-compose.yml up -d

# Tear down for local dev
local-down:
  @echo "Tearing Down Docker Services..."
  docker kill $(docker ps -q)

###############################################################################
# Fun Recipes
###############################################################################

# print the working directory that just recipes are executed from!
pwd:
  pwd
