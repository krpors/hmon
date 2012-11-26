package main

import (
	"encoding/xml"
	"testing"
)

// Just tests that unmarshalling a malformed config file should fail.
func TestWrongXml(t *testing.T) {
	// Simple parse error
	var badXml = `<?xml version="1.0" encoding="UTF-8"?>
<hmonconfig>
    <monitor name="Example.org index" desc="Checks iana.org example page.">
        <url>http://www.iana.org/domains/example/</url>
        <req>./env/request1.xml</req>
        <timeout>60</timeout>
        <headers>
            <header name="SOAPAction" value="whatevs"/>
        </headers>
        <assertions>
            <assertion>Example Domains</assertion>
        </assertions>
    </monitor/> <!-- parse error should occur here -->
</hmonconfig>`

	c := Config{}
	err := xml.Unmarshal([]byte(badXml), &c)
	if err == nil {
		t.Error("parsing should fail")
	}
}

// Tests normal parsing of the configuration, and asserts that the
// correct nodes are returned etc. Doesn't test all nodes.
func TestParse(t *testing.T) {
	// Correct config xml, with two monitors.
	var goodXml = `<?xml version="1.0" encoding="UTF-8"?>
<hmonconfig>
    <monitor name="first" desc="desc 1">
        <url>http://www.iana.org/domains/example/</url>
        <req>./env/request1.xml</req>
        <timeout>60</timeout>
        <headers>
            <header name="SOAPAction" value="whatevs"/>
        </headers>
        <assertions>
            <assertion>Example Domains</assertion>
            <assertion>Example Domains</assertion>
        </assertions>
    </monitor>
    <monitor name="second" desc="desc 2">
        <url>http://www.example.org/</url>
        <req>./env/request1.xml</req>
        <timeout>90</timeout>
        <headers>
            <header name="SOAPAction" value="ping"/>
            <header name="OtherHeader" value="somevalue"/>
        </headers>
        <assertions>
            <assertion>Example Domains</assertion>
        </assertions>
    </monitor>
</hmonconfig>`

	c := Config{}
	err := xml.Unmarshal([]byte(goodXml), &c)
	if err != nil {
		t.Error("unmarshalling failed: ", err)
	}

	if len(c.Monitor) != 2 {
		t.Errorf("expecting 2 monitors, got %d", len(c.Monitor))
	}

	if c.Monitor[0].Name != "first" {
		t.Errorf("requiring name '%s', got '%s'", "first", c.Monitor[0].Name)
	}

	if len(c.Monitor[0].Headers) != 1 {
		t.Errorf("expecting 1 header")
	}

	if len(c.Monitor[1].Headers) != 2 {
		t.Errorf("expecting 2 headers")
	}

	if len(c.Monitor[0].Assertions) != 2 {
		t.Errorf("expecting 2 assertions")
	}

	if len(c.Monitor[1].Assertions) != 1 {
		t.Errorf("expecting 1 assertion")
	}
}

func TestValidate(t *testing.T) {
	// this XML has a few incorrect regexes, and empty/faulty urls 
	// It should barf up 4 errors.
	var badXml = `<?xml version="1.0" encoding="UTF-8"?>
<hmonconfig>
    <monitor name="Example.org index" desc="Checks iana.org example page.">
        <url></url> <!-- empty, should fail -->
        <req></req>
        <timeout>60</timeout>
        <assertions>
            <assertion>^correct.*</assertion>
            <assertion>in(correct</assertion>
        </assertions>
    </monitor> 
    <monitor name="meh" desc="foo.">
        <url>h ttp://malformed</url> <!-- malformed url-->
        <req></req>
        <timeout>60</timeout>
        <assertions>
            <assertion>^correct.*</assertion>
            <assertion>(blah[)</assertion>
        </assertions>
    </monitor> 

</hmonconfig>`

	c := Config{}
	err := xml.Unmarshal([]byte(badXml), &c)
	if err != nil {
		t.Error("failed to parse xml: ", err)
	}

	err = c.Validate()
	if err == nil {
		t.Error("should run into error")
	}

	verr := err.(ValidationError)

	if len(verr.ErrorList) != 4 {
		t.Errorf("expected 4 errors, got %d", len(verr.ErrorList))
	}
}
