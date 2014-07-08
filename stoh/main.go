package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

/*
A SoapUI project has the following structure (only the elements
we're interested in are given):

soapui-project
	interface* (@name = binding name)
		operation* (@action = soapaction for this binding)
					@bindingOperationName = operation name)
	testSuite* (@name)
		testCase*
			testStep* (@name)
				config
					interface = the endpoint binding
					operation = the operation of the binding
					request (@name)
						endpoint (endpoint URL)
						request (cdata content with request)
						assertion @type ("Simple NotContains",  "Not Contains", "Simple Contains")
							configuration
								token ("Error")
								ignoreCase bool
								useRegEx bool
						wsaConfig (@action, unreliable)
*/

// Project is the root node of a SoapUI project.
type Project struct {
	Interface []Interface `xml:"interface"`           // all deffed interfaces
	TestSuite []TestSuite `xml:"testSuite"`           // all testsuites
	Property  []Property  `xml:"properties>property"` // project-wide properties
}

// Print prints out the full project to the given writer, in a
// structured view.
func (p Project) Print(writer io.Writer) {
	for _, i := range p.Interface {
		fmt.Fprintf(writer, "Interface '%s'\n", i.Name)
		for _, x := range i.Operation {
			fmt.Fprintf(writer, "\tOperation name:  %s\n", x.Name)
			fmt.Fprintf(writer, "\tSOAP Action:     %s\n\n", x.SoapAction)
		}
	}

	fmt.Fprintf(writer, "Project properties: %d\n", len(p.Property))
	for _, p := range p.Property {
		fmt.Fprintf(writer, "\t%s = %s\n", p.Name, p.Value)
	}

	fmt.Fprintln(writer)

	for _, s := range p.TestSuite {
		fmt.Fprintf(writer, "Testsuite '%s'\n", s.Name)
		fmt.Fprintf(writer, "\tTestsuite properties: %d\n", len(s.Property))
		for _, p := range s.Property {
			fmt.Fprintf(writer, "\t\t%s = %s\n", p.Name, p.Value)
		}

		fmt.Fprintln(writer)

		for _, t := range s.TestCase {
			fmt.Printf("\tTestcase '%s'\n", t.Name)
			fmt.Fprintf(writer, "\t\tTestcase properties: %d\n", len(t.Property))
			for _, p := range t.Property {
				fmt.Fprintf(writer, "\t\t\t%s = %s\n", p.Name, p.Value)
			}

			fmt.Fprintln(writer)

			for _, ts := range t.TestStep {
				fmt.Fprintf(writer, "\t\tName:        %s\n", ts.Name)
				fmt.Fprintf(writer, "\t\tType:        %s\n", ts.Type)
				fmt.Fprintf(writer, "\t\tTimeout:     %d ms\n", ts.Request.GetTimeout())

				if ts.Type == "request" {
					fmt.Fprintf(writer, "\t\tEndpoint:    %s\n", ts.Request.Endpoint)
					fmt.Fprintf(writer, "\t\tOperation:   %s\n", ts.Operation)
					fmt.Fprintf(writer, "\t\tBinding:     %s\n", ts.Binding)
					fmt.Fprintf(writer, "\t\tReq len:     %d\n", len(ts.Request.Content))
					fmt.Fprintf(writer, "\t\tSOAPAction:  %s\n", p.FindSoapAction(ts.Binding, ts.Operation))
					fmt.Fprintf(writer, "\t\tAssertions:  %d\n", len(ts.Request.Assertion))
					fmt.Fprintf(writer, "\t\t (valid):    %d\n", len(ts.Request.GetAssertions()))
				} else if ts.Type == "httprequest" {
					fmt.Fprintf(writer, "\t\tEndpoint:    %s\n", ts.Endpoint)
					fmt.Fprintf(writer, "\t\tReq len:     %d\n", len(ts.Request.Content2))
					fmt.Fprintf(writer, "\t\tAssertions:  %d\n", len(ts.Assertion))
					fmt.Fprintf(writer, "\t\t (valid):    %d\n", len(ts.GetAssertions()))
				}
				fmt.Fprintln(writer)
			}
		}
	}
}

// GetAllProperties finds all project propeties and puts them in a map.
// The keys of the map will have the following form:
//
//	${#Project#some.property.name}
//	${#Project#another.property.name}
//
// and so on.
func (p Project) GetAllProperties() map[string]string {
	m := make(map[string]string)

	// get all project wide properties
	for _, pp := range p.Property {
		key := fmt.Sprintf("${#Project#%s}", pp.Name)
		m[key] = pp.Value
	}

	return m
}

// FindSoapAction iterates through the interfaces and its operations to
// find the correct SOAPAction belonging to the binding name and operation
// name. This function is used to get the correct SOAP Action when processing
// testsuites/cases, since the SOAP action cannot be retrieved reliably from
// those elements and descendants.
func (p Project) FindSoapAction(bindingName, operationName string) string {
	for _, interf := range p.Interface {
		if interf.Name == bindingName {
			for _, operation := range interf.Operation {
				if operation.Name == operationName {
					return operation.SoapAction
				}
			}
		}
	}
	return ""
}

// Interface is a repeating element in the Project rootnode.
type Interface struct {
	Name      string      `xml:"name,attr"` // the binding name
	Operation []Operation `xml:"operation"` // operations per binding
}

// Operation is a repeating element in the Project rootnode.
type Operation struct {
	Name       string `xml:"name,attr"`   // op name
	SoapAction string `xml:"action,attr"` // soapaction for op
}

// TestSuite contains SoapUI testcases.
type TestSuite struct {
	Name     string     `xml:"name,attr"`
	TestCase []TestCase `xml:"testCase"`
	Property []Property `xml:"properties>property"`
}

func (t TestSuite) GetAllProperties() map[string]string {
	m := make(map[string]string)

	// get all testsuite properties
	for _, pp := range t.Property {
		key := fmt.Sprintf("${#TestSuite#%s}", pp.Name)
		m[key] = pp.Value
	}

	return m

}

// TestCase contains SoapUI test steps.
type TestCase struct {
	Name     string     `xml:"name,attr"`
	TestStep []TestStep `xml:"testStep"`
	Property []Property `xml:"properties>property"`
}

func (t TestCase) GetAllProperties() map[string]string {
	m := make(map[string]string)

	// get all testsuite properties
	for _, pp := range t.Property {
		key := fmt.Sprintf("${#TestCase#%s}", pp.Name)
		m[key] = pp.Value
	}

	return m

}

// TestStep contains information about teststeps within a testcase.
type TestStep struct {
	Name      string  `xml:"name,attr"`
	Type      string  `xml:"type,attr"`
	Binding   string  `xml:"config>interface"`
	Operation string  `xml:"config>operation"`
	Request   Request `xml:"config>request"`

	// Only in case type == "httprequest":
	Assertion []Assertion `xml:"config>assertion"`
	Endpoint  string      `xml:"config>endpoint"`
}

// GetAssertions find the correct assertions applicable for hmon. SoapUI defines
// several types of assertions (like Groovy scripts etc.) but we're only interested
// in the simple "Contains" assertions, since hmon can only assert against those.
// Well, also regular expressions, but thats a TODO.
func (ts TestStep) GetAssertions() []string {
	var validAssertions []string

	for _, ass := range ts.Assertion {
		if ass.Type == "Simple Contains" {
			validAssertions = append(validAssertions, ass.Token)
		}
	}
	return validAssertions
}

// GetSanitizedName sanitizes the name of a teststep so it can be used in the resulting
// toml configuration file. The current toml parser from BurntSushi does not accept periods
// in the name of map entries (e.g. [monitor.Blah.1.0] is invalid). Currently, the periods
// are replaced with an empty string.
func (ts TestStep) GetSanitizedName() string {
	return strings.Replace(ts.Name, ".", "", -1)
}

// Request contains information about the teststep request. The timeout is defined in an
// attribute, so we need to deserialize it in this struct.
type Request struct {
	Endpoint  string      `xml:"endpoint"`
	Content   string      `xml:"request"` // when SOAP, content is within the request element
	Timeout   int         `xml:"timeout,attr"`
	Assertion []Assertion `xml:"assertion"`

	// Only applicable when step type == "httprequest"
	Content2 string `xml:",chardata"` // when non-SOAP, content is contained within this tag :/
}

// GetAssertions find the correct assertions applicable for hmon. SoapUI defines
// several types of assertions (like Groovy scripts etc.) but we're only interested
// in the simple "Contains" assertions, since hmon can only assert against those.
// Well, also regular expressions, but thats a TODO.
func (req Request) GetAssertions() []string {
	var validAssertions []string

	for _, ass := range req.Assertion {
		if ass.Type == "Simple Contains" {
			validAssertions = append(validAssertions, ass.Token)
		}
	}
	return validAssertions
}

// GetTimeout returns a 'sane' timeout value. If it's not found, or lower than zero, the
// timeout will be set to a default of 30000 ms.
func (req Request) GetTimeout() int {
	if req.Timeout <= 0 {
		return 30000
	}

	return req.Timeout
}

// Assertion contains information about the teststep's assertions.
type Assertion struct {
	Type  string `xml:"type,attr"`
	Token string `xml:"configuration>token"`
}

// Property contains project, testsuite or testcase properties.
type Property struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

// ParseFile parses the given file to a Project struct. Will return
// an error if anything failed.
func ParseFile(file string) (Project, error) {
	p := Project{}

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return p, err
	}

	err = xml.Unmarshal(bytes, &p)
	if err != nil {
		return p, err
	}

	return p, nil
}

// MustCreateDir creates a directory denoted by the dir argument. If the directory
// cannot be created, an error is printed to stderr, and the program will exit.
func MustCreateDir(dir string) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directory: %s\n", err)
		os.Exit(2)
	}
}

// MustCreateFile creates an empty file denoted by the file argument and returns it.
// If the file cannot be created, an error is printed to stderr and will exit.
func MustCreateFile(file string) *os.File {
	outfile, err := os.Create(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file: %s\n", err)
		os.Exit(2)
	}

	return outfile
}

// SearchAndReplace searches in the given text for all the keys given in the map, and
// replaces them with the value belonging to that key.
func SearchAndReplace(text string, kvs map[string]string) string {
	for key, value := range kvs {
		text = strings.Replace(text, key, value, -1)
	}

	return text
}

// MergeMap is a small utility function to copy all elements from the src map to the dst map.
func MergeMap(src map[string]string, dst map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

// Process processes the given project and writes the generated output to the
// (current) fixed '_generated' directory.
func Process(p Project) {
	basedir := "_generated"
	configsdir := path.Join(basedir, "configs")
	postdatadir := path.Join(basedir, "postdata")

	MustCreateDir(configsdir)
	MustCreateDir(postdatadir)

	for _, s := range p.TestSuite {
		outfile := MustCreateFile(path.Join(configsdir, s.Name+"_hmon.toml"))

		testsuitePostdataDir := path.Join(postdatadir, s.Name)
		MustCreateDir(testsuitePostdataDir)

		fmt.Fprintf(outfile, "name = \"%s\"\n\n", s.Name)
		for _, c := range s.TestCase {
			// first, gather all possible properties for the underlying testcases
			properties := p.GetAllProperties()
			MergeMap(s.GetAllProperties(), properties)
			MergeMap(c.GetAllProperties(), properties)

			fmt.Println("MAP SIZE IS NOW", len(properties))

			for _, step := range c.TestStep {

				// write the request file
				postDataFile := MustCreateFile(path.Join(testsuitePostdataDir, step.Name+".xml"))
				if step.Type == "request" {
					fmt.Fprintf(postDataFile, SearchAndReplace(step.Request.Content, properties))
				} else if step.Type == "httprequest" {
					fmt.Fprintf(postDataFile, SearchAndReplace(step.Request.Content2, properties))
				}
				postDataFile.Close()

				fmt.Fprintf(outfile, "[monitor.%s]\n", step.GetSanitizedName())
				fmt.Fprintf(outfile, "name = \"%s\"\n", step.Name)
				fmt.Fprintf(outfile, "file = \"%s/%s.xml\"\n", s.Name, step.Name)
				fmt.Fprintf(outfile, "timeout = %d\n", step.Request.GetTimeout())

				if step.Type == "request" {
					fmt.Fprintf(outfile, "url = \"%s\"\n", SearchAndReplace(step.Request.Endpoint, properties))
					fmt.Fprintf(outfile, "headers = [\n")
					fmt.Fprintf(outfile, "  \"SOAPAction: %s\",\n", p.FindSoapAction(step.Binding, step.Operation))
					fmt.Fprintf(outfile, "  \"Content-Type: %s\"\n", "application/soap+xml")
					fmt.Fprintf(outfile, "]\n")
					fmt.Fprintf(outfile, "assertions = [\n")
					for _, ass := range step.Request.GetAssertions() {
						fmt.Fprintf(outfile, "  \"%s\",\n", ass)
					}
					fmt.Fprintf(outfile, "]\n")

				} else if step.Type == "httprequest" {
					fmt.Fprintf(outfile, "url = \"%s\"\n", SearchAndReplace(step.Endpoint, properties))
					fmt.Fprintf(outfile, "assertions = [\n")
					for _, ass := range step.GetAssertions() {
						fmt.Fprintf(outfile, "  \"%s\",\n", ass)
					}
					fmt.Fprintf(outfile, "]\n")
				}

				fmt.Fprintln(outfile)
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Expecting one argument (SoapUI project file with a testsuite)\n")
		os.Exit(1)
	}

	project, err := ParseFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse project file: %s\n", err)
		os.Exit(1)
	}

	Process(project)
	//project.Print(os.Stdout)
}
