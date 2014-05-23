package main

import (
	"testing"
)

// Simply prepares a project file, serialized as structs to test with.
func prepareProject() Project {
	p := Project{
		TestSuite: []TestSuite{
			{
				Name: "TestSuite One",
				TestCase: []TestCase{
					{
						Name: "GetRelation Tests",
						TestStep: []TestStep{
							{
								Name:      "Step 1",
								Binding:   "GetRelation1.0-EndpointBinding",
								Operation: "getRelationName",
								Request: Request{
									Endpoint: "http://example.org:80/getRelationName/1.0",
									Content:  "<soapenv:Envelope> ... </soapenv:Envelope>",
									Timeout:  0,
									Assertion: []Assertion{
										{
											Type:  "Simple Contains",
											Token: "Text in response",
										},
										{
											Type:  "Simple Contains",
											Token: "Other text",
										},
										{
											// should be ignored for now (also, not sure
											// if correct Type. Figure this out)
											Type:  "RegEx",
											Token: "^whatevs$",
										},
										{
											// should be ignored
											Type:  "Groovy Script",
											Token: "slkjdalksj",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Interface: []Interface{
			{
				Name: "GetRelation-1.0EndpointBinding",
				Operation: []Operation{
					{
						Name:       "getRelationName",
						SoapAction: "http://example.org/getRelationName",
					},
					{
						Name:       "getRelationAge",
						SoapAction: "http://example.org/getRelationAge",
					},
				},
			},
			{
				Name: "Authenticate-1.0EndpointBinding",
				Operation: []Operation{
					{
						Name:       "authenticateUserPass",
						SoapAction: "http://example.org/authenticateUserPass",
					},
					{
						Name:       "authenticateFake",
						SoapAction: "http://example.org/authenticateFake",
					},
				},
			},
		},
	}

	return p
}

func TestFindSoapAction(t *testing.T) {
	p := prepareProject()

	assert := func(expected, actual string) {
		if expected != actual {
			t.Errorf("Expected '%s', got '%s'", expected, actual)
		}
	}

	action := p.FindSoapAction("GetRelation-1.0EndpointBinding", "getRelationAge")
	expected := "http://example.org/getRelationAge"
	assert(expected, action)

	action = p.FindSoapAction("GetRelation-1.0EndpointBinding", "getRelationName")
	expected = "http://example.org/getRelationName"
	assert(expected, action)

	action = p.FindSoapAction("Authenticate-1.0EndpointBinding", "authenticateFake")
	expected = "http://example.org/authenticateFake"
	assert(expected, action)
}

func TestGetAssertions(t *testing.T) {
	p := prepareProject()

	testStep := p.TestSuite[0].TestCase[0].TestStep[0]
	assertions := testStep.Request.GetAssertions()

	if len(assertions) != 2 {
		t.Errorf("Expected 2 assertions, got %d", len(assertions))
	}

	expected := "Text in response"
	if assertions[0] != expected {
		t.Errorf("First assertion should be '%s', got '%s'", expected, assertions[0])
	}
	expected = "Other text"
	if assertions[1] != expected {
		t.Errorf("Second assertion should be '%s', got '%s'", expected, assertions[1])
	}
}

func TestGetTimeout(t* testing.T) {
	p := prepareProject()
	request := p.TestSuite[0].TestCase[0].TestStep[0].Request

	timeout := request.GetTimeout()
	if timeout != 30000 {
		t.Errorf("Expected a default of 30000 ms when timeout is not specified")
	}
}
