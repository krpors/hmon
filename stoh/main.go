package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

/*
soapui-project
	interface* (@name = de binding)
		operation* (@action = soapaction voor deze binding
					@bindingOperationName = operation name)
	testSuite* (@name)
		testCase*
			testStep* (@name)
				config
					interace = de endpoint binding
					operatoin = de operation van de binding
					request (@name, bijv. Authenticate)
						endpoint (url hierin)
						request (cdata content met request)
						assertion @type ("Simple NotContains",  "Not Contains")
							configuration
								token ( "Error" )
								ignoreCase bool
								useRegEx bool
						wsaConfig (@action)
*/

type Project struct {
	Interface []Interface `xml:"interface"` // all deffed interfaces
	TestSuite []TestSuite `xml:"testSuite"`
}

// Print prints out the full project to the given writer in a
// structured view.
func (this Project) Print(writer io.Writer) {
	for _, i := range this.Interface {
		fmt.Fprintf(writer, "Interface '%s'\n", i.Name)
		for _, x := range i.Operation {
			fmt.Fprintf(writer, "\tOperation name:  %s\n", x.Name)
			fmt.Fprintf(writer, "\tSOAP Action:     %s\n\n", x.SoapAction)
		}
	}

	for _, s := range this.TestSuite {
		fmt.Fprintf(writer, "Testsuite '%s'\n", s.Name)
		for _, t := range s.TestCase {
			fmt.Printf("\tTestcase '%s'\n", t.Name)
			for _, ts := range t.TestStep {
				fmt.Fprintf(writer, "\t\tName:        %s\n", ts.Name)
				fmt.Fprintf(writer, "\t\tEndpoint:    %s\n", ts.Endpoint)
				fmt.Fprintf(writer, "\t\tOperation:   %s\n", ts.Operation)
				fmt.Fprintf(writer, "\t\tBinding:     %s\n", ts.Binding)
				fmt.Fprintf(writer, "\t\tReq len:     %d\n", len(ts.Request))
				fmt.Fprintf(writer, "\t\tAssertions:  %d\n", len(ts.Assertion))
				fmt.Fprintf(writer, "\t\t (valid):    %d\n", len(ts.GetAssertions()))
				fmt.Fprintln(writer)
			}
		}
	}
}

// FindSoapAction iterates through the interfaces and its operations to
// find the correct SOAPAction belonging to the binding name and operation
// name. This function is used to get the correct SOAP Action when processing
// testsuites/cases, since the SOAP action cannot be retrieved reliably from
// those elements and descendants.
func (this Project) FindSoapAction(bindingName, operationName string) string {
	for _, interf := range this.Interface {
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

type Interface struct {
	Name      string      `xml:"name,attr"` // the binding name
	Operation []Operation `xml:"operation"` // operations per binding
}

type Operation struct {
	Name       string `xml:"name,attr"`   // op name
	SoapAction string `xml:"action,attr"` // soapaction for op
}

type TestSuite struct {
	Name     string     `xml:"name,attr"`
	TestCase []TestCase `xml:"testCase"`
}

type TestCase struct {
	Name     string     `xml:"name,attr"`
	TestStep []TestStep `xml:"testStep"`
}

type TestStep struct {
	Name      string      `xml:"name,attr"`
	Endpoint  string      `xml:"config>request>endpoint"`
	Request   string      `xml:"config>request>request"`
	Binding   string      `xml:"config>interface"`
	Operation string      `xml:"config>operation"` // TODO crossref operation/binding with con:interface to get the correct soapaction
	Assertion []Assertion `xml:"config>request>assertion"`
}

// GetAssertions find the correct assertions applicable for hmon. SoapUI defines
// several types of assertions (like Groovy scripts etc.) but we're only interested
// in the simple "Contains" assertions, since hmon can only assert against those.
// Well, also regular expressions, but thats a TODO.
func (this TestStep) GetAssertions() []string {
	var validAssertions []string

	for _, ass := range this.Assertion {
		if ass.Type == "Simple Contains" {
			validAssertions = append(validAssertions, ass.Token)
		}
	}
	return validAssertions
}

type Assertion struct {
	Type  string `xml:"type,attr"`
	Token string `xml:"configuration>token"`
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

func main() {
	project, err := ParseFile("Demo-soapui-project.xml")
	if err != nil {
		fmt.Println(err)
		return
	}

	project.Print(os.Stdout)
}
