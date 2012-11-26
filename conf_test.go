package main

import (
	"encoding/xml"
	"testing"
)

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
</hmonconfig>
`

var goodXml = `<?xml version="1.0" encoding="UTF-8"?>
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
    </monitor>
    <monitor name="Some site" desc="Checks something else.">
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
</hmonconfig>
`

func TestWrongXml(t *testing.T) {
	c := Config{}
	err := xml.Unmarshal([]byte(badXml), &c)
	if err == nil {
		t.Error("parsing should fail")
	}
}

func TestParse(t *testing.T) {
	c := Config{}
	err := xml.Unmarshal([]byte(goodXml), &c)
	if err != nil {
		t.Error("unmarshalling failed: ", err)
	}
}

