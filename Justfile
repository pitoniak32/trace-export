set dotenv-load

run:
  go run main.go

test: 
  go test

sloc:
  @echo "`wc -l *.go` lines of code"
