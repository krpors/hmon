package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
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
	TestSuite []TestSuite `xml:"testSuite"`
	Interface []Interface `xml:"interface"` // all deffed interfaces
}

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
	var	validAssertions []string

	for _, ass := range this.Assertion {
		if ass.Type == "Simple Contains" {
			validAssertions = append(validAssertions, ass.Token)
		}
	}
	return validAssertions
}


type Assertion struct {
	Type  string `xml:"type"`
	Token string `xml:"configuration>token"`
}

// PrintStructure prints the (assumed) SoapUI project file given in 'file', to
// the standard output.
func PrintStructure(p Project) {
	for _, i := range p.Interface {
		fmt.Println("Interface: ", i.Name)
		for _, x := range i.Operation {
			fmt.Printf("\tOpname: %s\n", x.Name)
			fmt.Printf("\tAction: %s\n\n", x.SoapAction)
		}
	}

	for _, s := range p.TestSuite {
		fmt.Println(s.Name)
		for _, t := range s.TestCase {
			fmt.Println("\t", t.Name)
			for _, ts := range t.TestStep {
				fmt.Printf("\t\tName:        %s\n", ts.Name)
				fmt.Printf("\t\tEndpoint:    %s\n", ts.Endpoint)
				fmt.Printf("\t\tOperation:   %s\n", ts.Operation)
				fmt.Printf("\t\tBinding:     %s\n", ts.Binding)
				fmt.Printf("\t\tReq len:     %d\n", len(ts.Request))
				fmt.Printf("\t\tAssertions:  %d\n", len(ts.Assertion))
				fmt.Println()
			}
		}
	}
}

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

	PrintStructure(project)
}
