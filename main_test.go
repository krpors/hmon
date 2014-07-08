package main

import (
	"testing"
)

func TestSanitize(t *testing.T) {
	s := "Some escapable string"
	result := sanitizePandoraData(s)
	if result != "Some escapable string" {
		t.Errorf("Unexpected: '%s'", result)
	}

	s = "Assertion failed for regex `hi thar'"
	result = sanitizePandoraData(s)
	if result != "Assertion failed for regex hi thar" {
		t.Errorf("Unexpected: '%s'", result)
	}

	s = "``````'''''unlimited backticks'''' and'''``\"\" quotes"
	result = sanitizePandoraData(s)
	if result != "unlimited backticks and quotes" {
		t.Errorf("Unexpected: '%s'", result) 
	}
}
