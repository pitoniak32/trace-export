package roll

import (
	"fmt"
	"testing"
)

func TestNothing(t *testing.T) {
	fmt.Printf("running test")
	t.Fatalf("Oppsie!")
}
