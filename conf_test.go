package main

import (
	"testing"
)

// Tests normal parsing of the configuration, and asserts that the
// correct nodes are returned etc. Doesn't test all nodes.
func TestParse(t *testing.T) {
	// TODO: test parsing, validation, etc.
}

// Tests the splitting of headers and such.
func TestHeaders(t *testing.T) {
	type Exp struct {
		FullHeader    Header
		ExpectedName  string
		ExpectedValue string
	}

	tests := []Exp{
		{"SOAPAction: http://www.example.org/bogus/soapaction", "SOAPAction", "http://www.example.org/bogus/soapaction"},
		{"Content-Type: text/xml", "Content-Type", "text/xml"},
		{"Content-Type:application/json", "Content-Type", "application/json"},
		{"Content-Length:      22222", "Content-Length", "22222"},
	}

	for _, test := range tests {
		pn := test.FullHeader.GetName()
		pv := test.FullHeader.GetValue()

		if pn != test.ExpectedName {
			t.Errorf("expected '%s', got '%s'!", test.ExpectedName, pn)
		}

		if pv != test.ExpectedValue {
			t.Errorf("expected '%s', got '%s'!", test.ExpectedValue, pv)
		}
	}
}

func TestHeaderValidate(t *testing.T) {
	header := Header("Header no colon")

	err := header.Validate()
	if err == nil {
		t.Errorf("expected error on header '%s'", header)
	}

	header = Header("Name with space: some value")
	err = header.Validate()
	if err == nil {
		t.Errorf("expected error on header '%s'", header)
	}
}
