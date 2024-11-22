# This will ensure `.env` file is evaluated before recipes are run!
set dotenv-load

# NOTE: these will be executed from project root!
# for a sanity check try `just pwd` from a non root dir.

# Run the entry point
run:
  go run main.go

# test all packages
test: 
  go test ./...

# lint all packages
lint:
  golangci-lint run ./..

# format all packages
fmt:
  go fmt ./...

###############################################################################
# Fun Recipes
###############################################################################

# print the working directory that just recipes are executed from!
pwd:
  pwd