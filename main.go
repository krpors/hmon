package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
)

var (
	confdir      = flag.String("confdir", ".", "Directory with configurations of *_hmon.xml files.")
	reqdir       = flag.String("reqdir", ".", "Base directory to search for request files. If ommited, the current working directory is used.")
	validateOnly = flag.Bool("validate", false, "When specified, only validate the configuration file(s), but don't run the monitors.")
)

// Type Result encapsulates information about a Monitor and its invocation result. 
type Result struct {
	Monitor *Monitor // the monitor which may or may not have failed
	Valid   bool     // ok or not ok?
	Error   error    // An error, describing the possible failure. If empty, it's ok.
}

// Returns the result as a string for some easy-peasy debuggin'.
func (r Result) String() string {
	if r.Error == nil {
		return fmt.Sprintf("ok    %s", r.Monitor.Name)
	}

	return fmt.Sprintf("FAIL  %s: %s", r.Monitor.Name, r.Error)
}

// timeout dialer, see https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/2ehqb6t54kA

// Runs a check for the given Monitor. There are a few things done in this function.
// If the given input file is empty (i.e. none), a http GET is issued to the given URL.
// If a file is given though, this will become a http POST, with the post-data being the
// file's contents. If there are any assertions configured, all the assertions are used
// to test the content. If none are configured, it will just be a sort of 'ping-check',
// i.e. checking if a connection could be made to the URL.
func runCheck(m *Monitor, baseDir string, c chan Result) {
	client := http.Client{}

	// when no file is specified, do a GET
	var req *http.Request
	var err error

	if m.File == "" {
		req, err = http.NewRequest("GET", m.Url, nil)
	} else {
		requestBody, err := ioutil.ReadFile(path.Join(baseDir, m.File))
		if err != nil {
			c <- Result{m, false, err}
			return
		}
		req, err = http.NewRequest("POST", m.Url, bytes.NewReader(requestBody))
	}

	if err != nil {
		c <- Result{m, false, err}
		return
	}

	// add all optional headers:
	for i := range m.Headers {
		req.Header.Add(m.Headers[i].Name, m.Headers[i].Value)
	}

	resp, err := client.Do(req)
	if err != nil {
		c <- Result{m, false, err}
		return
	}
	defer resp.Body.Close()

	responseContents, err := ioutil.ReadAll(resp.Body)

	// whether the response validates against the assertions.
	// When no assertions are given, just check if the site/host is up.
	for i := range m.Assertions {
		// at this point, compilation of the regular expression must succeed,
		// since we already executed a Validate() on the configuration itself.
		// To make things sure, we do a MustCompile though.
		rex := regexp.MustCompile(m.Assertions[i])
		found := rex.Find(responseContents)
		if found == nil {
			c <- Result{m, false, fmt.Errorf("assertion failed for regex `%s'", m.Assertions[i])}
			return
		}
	}

	// passed all tests, return true to the channel
	c <- Result{m, true, nil}
}

func localtest() {
	monitor := Monitor{}
	monitor.Name = "Test"
	monitor.Description = "Test"
	monitor.Url = "http://omgwtfbbqz.nl"
	monitor.Assertions = append(monitor.Assertions, "teaaaast")

	ch := make(chan Result, 1)

	runCheck(&monitor, "", ch)

	fmt.Println(<-ch)
}

// Validates all configurations in the slice. For every failed validation,
// print it out to stdout. If any failures occured, simply bail out.
func validateConfigurations(configurations *[]Config) {
	if len(*configurations) == 0 {
		fmt.Printf("No configurations found were found in `%s'\n", *confdir)
		fmt.Printf("Note that only files with suffix *_hmon.xml are parsed.\n")
		os.Exit(1)
	}

	// boolean indicating that configurations are not valid.
	var success bool = true
	var totalerrs int8 = 0

	for i := range *configurations {
		c := (*configurations)[i]
		err := c.Validate()
		if err != nil {
			// we got validation errors.
			verr := err.(ValidationError)
			fmt.Printf("%s: %s\n", c.FileName, verr)
			for i := range verr.ErrorList {
				fmt.Printf("  %s\n", verr.ErrorList[i])
				totalerrs++
			}

			success = false
		}
	}

	if !success {
		fmt.Printf("\nFailed due to a total of %d validation errors.\n", totalerrs)
		os.Exit(1)
	}

	// Is a flag provided that we only should do configuration validation?
	if *validateOnly {
		// if so, no point in continuing. Exit code 0 to indicate an a-okay.
		fmt.Println("ok")
		os.Exit(0)
	}
}

func main() {
	flag.Parse()

	// First, find the configurations from the confdir. Bail if anything fails.
	configurations, err := FindConfigs(*confdir)
	if err != nil {
		fmt.Printf("Unable to find/parse configuration files. Nested error is: %s\n", err)
		os.Exit(1)
	}

	validateConfigurations(&configurations)

	_, err = os.Open(*reqdir)
	if err != nil {
		fmt.Printf("Failed to open request directory. Nested error is: %s\n", err)
		os.Exit(1)
	}

	for _, c := range configurations {
		fmt.Println("Running monitors:", c.Name)

		// receiver channel
		ch := make(chan Result, len(c.Monitors))

		for i := range c.Monitors {
			go runCheck(&c.Monitors[i], *reqdir, ch)
		}

		// read from the channel until all monitors have sent their response
		for _ = range c.Monitors {
			fmt.Println(<-ch)
		}

	}
}
