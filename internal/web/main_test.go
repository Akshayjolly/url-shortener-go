package web

import (
	"io"
	"log"
	"os"
	"testing"
)

// TestMain silences request logging so test output stays readable.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}
