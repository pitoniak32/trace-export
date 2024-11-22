# This will ensure `.env` file is evaluated before recipes are run!
set dotenv-load

# NOTE: these will be executed from project root!
# for a sanity check try `just pwd` from a non root dir.

# Run the entry point
run:
  go run main.go

# Test all packages
test: 
  go test ./...

# Lint all packages
lint: fmt
  golangci-lint run ./..

# Format all packages
fmt:
  go fmt ./...

# Setup supporting services
local-setup:
  docker compose -f ./docker-compose.yml up -d

# Tear down for local dev
local-teardown:
  docker kill $(docker ps -q)

###############################################################################
# Fun Recipes
###############################################################################

# print the working directory that just recipes are executed from!
pwd:
  pwd