FROM golang:1.23.3 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o trace-export

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/trace-export /
CMD ["/trace-export"]