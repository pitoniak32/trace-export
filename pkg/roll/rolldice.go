package roll

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const name = "go.opentelemetry.io/otel/example/dice"

var (
	tracer = otel.Tracer(name)
)

func Rolldice(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "roll")
	defer span.End()

	roll := 1 + rand.Intn(6)

	var msg string
	if player := r.PathValue("player"); player != "" {
		msg = fmt.Sprintf("%s is rolling the dice", player)
	} else {
		msg = "Anonymous player is rolling the dice"
	}

	rollValueAttr := attribute.Int("roll.value", roll)
	span.SetAttributes(rollValueAttr)

	span.AddEvent(msg)

	resp := strconv.Itoa(roll) + "\n"
	if _, err := io.WriteString(w, resp); err != nil {
		log.Printf("Write failed: %v\n", err)
	}
}
