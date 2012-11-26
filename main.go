package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
)

// timeout dialer, see https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/2ehqb6t54kA

// future flags:
// - conf file
// - basedir for request files

// Runs a check for the given monitor. TODO: the channel must be something better,
// with more informative stuff.
func runCheck(m Monitor, c chan bool) {
	client := http.Client{}
	requestBody, err := ioutil.ReadFile(m.File)
	if err != nil {
		c <- false
	}
	req, err := http.NewRequest("POST", m.Url, bytes.NewReader(requestBody))
	// add all optional headers:
	for _, header := range m.Headers {
		req.Header.Add(header.Name, header.Value)
	}

	resp, err := client.Do(req)
	defer resp.Body.Close()

	responseContents, err := ioutil.ReadAll(resp.Body)

	// whether the response validates against the assertions
	for _, re := range m.Assertions {
		// at this point, compilation of the regular expression must succeed,
		// since we already executed a Validate() on the configuration itself.
		// To make things sure, we do a MustCompile though.
		rex := regexp.MustCompile(re)
		found := rex.Find(responseContents)
		if found == nil {
			c <- false
			return
		}
	}

	c <- true
}

func main() {
	contents, err := ioutil.ReadFile("conf.xml")
	if err != nil {
		fmt.Errorf("Fail!")
		os.Exit(1)
	}

	c := Config{}
	err = xml.Unmarshal(contents, &c)
	if err != nil {
		fmt.Errorf(err.Error())
	}

	err = c.Validate()
	if err != nil {
		verr := err.(ValidationError)
		fmt.Println(verr)
		for _, e := range verr.ErrorList {
			fmt.Println("  ", e)
		}

		os.Exit(2)
	}

	ch := make(chan bool, len(c.Monitor))

	// commence
	for _, monitor := range c.Monitor {
		go runCheck(monitor, ch)
	}

	for i := 0; i < len(c.Monitor); i++ {
		fmt.Println(<-ch)
	}
}
