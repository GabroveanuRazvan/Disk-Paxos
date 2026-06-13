package logger

import "testing"

func TestNewConsoleLogger(t *testing.T) {
	log, err := NewConsoleLogger()
	if err != nil {
		t.Fatalf("NewConsoleLogger returned error: %v", err)
	}
	if log == nil {
		t.Fatal("NewConsoleLogger returned nil logger")
	}
}
