package main

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

// Just tests that unmarshalling a malformed config file should fail.
func TestWrongXml(t *testing.T) {
	// Simple parse error
	var badXml = `<?xml version="1.0" encoding="UTF-8"?>
<hmonconfig name="hehe">
    <monitor name="Example.org index" desc="Checks iana.org example page.">
        <url>http://www.iana.org/domains/example/</url>
        <file>./env/request1.xml</file>
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
<hmonconfig name="hehe">
    <monitor name="first" desc="desc 1">
        <url>http://www.iana.org/domains/example/</url>
        <file>./env/request1.xml</file>
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
        <file>./env/request1.xml</file>
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

	if len(c.Monitors) != 2 {
		t.Errorf("expecting 2 monitors, got %d", len(c.Monitors))
	}

	if c.Monitors[0].Name != "first" {
		t.Errorf("requiring name '%s', got '%s'", "first", c.Monitors[0].Name)
	}

	if len(c.Monitors[0].Headers) != 1 {
		t.Errorf("expecting 1 header")
	}

	if len(c.Monitors[1].Headers) != 2 {
		t.Errorf("expecting 2 headers")
	}

	if len(c.Monitors[0].Assertions) != 2 {
		t.Errorf("expecting 2 assertions")
	}

	if len(c.Monitors[1].Assertions) != 1 {
		t.Errorf("expecting 1 assertion")
	}

	if (c.Name != "hehe") {
		t.Errorf("expecting montior name `hehe', got `%s`", c.Name)
	}
}

func TestValidate(t *testing.T) {
	// this XML has a few incorrect regexes, and empty/faulty urls 
	// It should barf up 5 errors.
	var badXml = `<?xml version="1.0" encoding="UTF-8"?>
<hmonconfig> <!-- missing name attribute here -->
    <monitor name="Example.org index" desc="Checks iana.org example page.">
        <url></url> <!-- empty, should fail -->
        <file></file>
        <timeout>60</timeout>
        <assertions>
            <assertion>^correct.*</assertion>
            <assertion>in(correct</assertion>
        </assertions>
    </monitor> 
    <monitor name="meh" desc="foo.">
        <url>h ttp://malformed</url> <!-- malformed url-->
        <file></file>
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

	if len(verr.ErrorList) != 5 {
		t.Errorf("expected 5 errors, got %d", len(verr.ErrorList))
	}
}

// This test function write two configuration files called groupone_hmon.xml
// and grouptwo_hmon.xml in the OS's/environment's temporary directory. This
// test then expects two correctly parsed configuration files.
//
// The two configuration files are automatically deleted.
func TestFindConfigs(t *testing.T) {
	// non existant filename. Should run into error.
	_, err := FindConfigs("0101010.0101")
	if err == nil {
		t.Error("expecting an error, got nil")
	}

	// write some temp configs in /tmp/. Then run FindConfigs
	// there to parse them?
	var goodXml = `<?xml version="1.0" encoding="UTF-8"?>
<hmonconfig name="MiMaMeh">
    <monitor name="first" desc="desc 1">
        <url>http://www.iana.org/domains/example/</url>
        <file>./env/request1.xml</file>
        <timeout>60</timeout>
        <headers>
            <header name="SOAPAction" value="whatevs"/>
        </headers>
        <assertions>
            <assertion>Example Domains</assertion>
        </assertions>
    </monitor>
</hmonconfig>
`
	file1 := path.Join(os.TempDir(), "groupone_hmon.xml")
	file2 := path.Join(os.TempDir(), "grouptwo_hmon.xml")
	err = ioutil.WriteFile(file1, []byte(goodXml), 0644)
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(file1)
	err = ioutil.WriteFile(file2, []byte(goodXml), 0644)
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(file2)

	// files created, run the Find. Should result in two files.
	configs, err := FindConfigs(os.TempDir())
	if err != nil {
		t.Error(err)
	}

	if len(configs) != 2 {
		t.Errorf("expected 2 configurations, got %d", len(configs))
	}

	// check validity as well just in case.
	for _, config := range configs {
		err = config.Validate()
		if err != nil {
			t.Errorf("validation failed for configuration")
		}
	}
}
