package main

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// ValidationError, used to return when validation fails.
type ValidationError struct {
	// Just an error list with all validation errors.
	ErrorList []string
}

// Simply appends the string to the stack
func (e *ValidationError) Add(s string) {
	e.ErrorList = append(e.ErrorList, s)
}

// Returns a summary of the errors as a one-liner.
func (e ValidationError) Error() string {
	var plural string = "error"
	if len(e.ErrorList) > 1 {
		plural = "errors"
	}
	return fmt.Sprintf("%d config validation %s found", len(e.ErrorList), plural)
}

// Root configuration node, contains zero or more Monitor
// structs.
type Config struct {
	// The original filename (basename)
	FileName string

	Name     string    `xml:"name,attr"`
	Monitors []Monitor `xml:"monitor"`
}

// Monitor node with its children. All slices can be zero or more,
// technically, but logically some kind of validation can be done using
// Validate().
type Monitor struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"desc,attr"`
	Url         string   `xml:"url"`
	File        string   `xml:"file"`
	Timeout     int      `xml:"timeout"`
	Headers     []Header `xml:"headers>header"`
	Assertions  []string `xml:"assertions>assertion"`
}

// timeout dialer, see https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/2ehqb6t54kA

// Runs a check for the given Monitor. There are a few things done in this function.
// If the given input file is empty (i.e. none), a http GET is issued to the given URL.
// If a file is given though, this will become a http POST, with the post-data being the
// file's contents. If there are any assertions configured, all the assertions are used
// to test the content. If none are configured, it will just be a sort of 'ping-check',
// i.e. checking if a connection could be made to the URL.
//
//  TODO is this pointer receiver really necessary? I don't think so. We're not changing
// the `m' anyway.
func (m *Monitor) Run(baseDir string, c chan Result) {
	client := http.Client{}

	// when no file is specified, do a GET
	var req *http.Request
	var err error

	if m.File == "" {
		req, err = http.NewRequest("GET", m.Url, nil)
	} else {
		requestBody, err := ioutil.ReadFile(path.Join(baseDir, m.File))
		if err != nil {
			c <- Result{m, 0, err}
			return
		}
		req, err = http.NewRequest("POST", m.Url, bytes.NewReader(requestBody))
	}

	if err != nil {
		c <- Result{m, 0, err}
		return
	}

	// add all optional headers:
	for i := range m.Headers {
		req.Header.Add(m.Headers[i].Name, m.Headers[i].Value)
	}

	// start measuring time from this point:
	tstart := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		c <- Result{m, 0, err}
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
			c <- Result{m, time.Now().Sub(tstart), fmt.Errorf("assertion failed for regex `%s'", m.Assertions[i])}
			return
		}
	}

	// passed all tests, return true to the channel
	c <- Result{m, time.Now().Sub(tstart), nil}
}

func (m Monitor) String() string {
	return fmt.Sprintf("%s (%s), %d headers, %d assertions", m.Name, m.Url, len(m.Headers), len(m.Assertions))
}

// Extra HTTP headers to send.
type Header struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Runs a validation over the parsed configuration file. The returned
// error is of type ValidationError.
func (c *Config) Validate() error {
	verr := ValidationError{}

	if strings.TrimSpace(c.Name) == "" {
		verr.Add("root node hmonconfig requires a non-empty name attribute")
	}

	for monidx := range c.Monitors {
		mon := c.Monitors[monidx]
		if mon.Name == "" {
			verr.Add(fmt.Sprintf("monitor[%d]: must have a non-empty name attribute", monidx))
		}

		if mon.Url == "" {
			verr.Add(fmt.Sprintf("monitor[%d]: empty url", monidx))
		} else {
			_, err := url.ParseRequestURI(mon.Url)
			if err != nil {
				verr.Add(fmt.Sprintf("monitor[%d]: malformed url: %s", monidx, err))
			}
		}

		for assidx := range mon.Assertions {
			_, err := regexp.Compile(mon.Assertions[assidx])
			if err != nil {
				verr.Add(fmt.Sprintf("monitor[%d]/assertion[%d]: invalid regex: %s", monidx, assidx, err))
			}
		}
	}

	// if we found 0 or more errors, return the verr, else ...
	if len(verr.ErrorList) > 0 {
		return verr
	}

	// ... return a nil error
	return nil
}

// Using a base directory, finds all configuration XML files and parses them.
// A slice of Configs are returned. Obviously, if the slice length is zero,
// and the error is non-nil, no configurations are found.
func FindConfigs(baseDir string) ([]Config, error) {
	dir, err := os.Open(baseDir)
	if err != nil {
		return nil, err
	}

	finfo, err := dir.Stat()
	if !finfo.IsDir() {
		return nil, fmt.Errorf("`%s' is not a directory", baseDir)
	}

	finfos, err := dir.Readdir(0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files from `%s'", baseDir)
	}

	var configurations = make([]Config, 0)

	for _, fi := range finfos {
		// only fetch files
		if !fi.IsDir() {
			if strings.HasSuffix(fi.Name(), "hmon.xml") {
				fullFile := path.Join(baseDir, fi.Name())
				contents, err := ioutil.ReadFile(fullFile)
				if err != nil {
					fmt.Println(err)
					continue
				}

				c := Config{}
				c.FileName = fi.Name()
				err = xml.Unmarshal(contents, &c)
				if err != nil {
					// when one or more config files can't be
					// parsed, bail out!
					return nil, fmt.Errorf("failed to parse file `%s': %s", fullFile, err)
				}

				// else we can just add it to the parsed configurations
				// slice and continue.
				configurations = append(configurations, c)
			}
		}
	}

	return configurations, nil
}

///////////////////////////////////////////////////////////////////////////////

// Type Result encapsulates information about a Monitor and its invocation result. 
type Result struct {
	Monitor *Monitor      // the monitor which may or may not have failed.
	Latency time.Duration // The latency of the call i.e. how long did it take.
	Error   error         // An error, describing the possible failure. If nil, it's ok.
}

// Returns the result as a string for some easy-peasy debuggin'.
func (r Result) String() string {
	if r.Error == nil {
		return fmt.Sprintf("ok    %s (%v)", r.Monitor.Name, r.Latency)
	}

	if r.Latency > 0 {
		return fmt.Sprintf("FAIL  %s: %s (%v)", r.Monitor.Name, r.Error, r.Latency)
	}

	return fmt.Sprintf("FAIL  %s: %s", r.Monitor.Name, r.Error)
}

// The ResultProcessor interface defines functions that processes Results to whatever.
type ResultProcessor interface {
	// Invoked when the monitors are run.
	Started()
	// Invoked to process a Config.
	ProcessConfig(c *Config)
	// Processes a Result.
	ProcessResult(r *Result)
	// Invoked when the monitors are finished.
	Finished()
}

// Default processor (outputs default stuff to stdout).
type DefaultProcessor struct {
	countOk   int16 // amount of OKs
	countFail int16 // amount of failures
}

func (p *DefaultProcessor) Started() {
}

func (p *DefaultProcessor) ProcessConfig(c *Config) {
	fmt.Printf("Processing config `%s'\n", (*c).Name)
}

func (p *DefaultProcessor) ProcessResult(r *Result) {
	if r.Error == nil {
		p.countOk++
	} else {
		p.countFail++
	}
	fmt.Printf("%s\n", *r)
}

func (p *DefaultProcessor) Finished() {
	fmt.Printf("\nFinished with %d successes and %d errors.\n", p.countOk, p.countFail)
}

// Outputs Results to CSV format on stdout.
type CsvProcessor struct {
	writer     *csv.Writer
	currConfig *Config
}

func (p *CsvProcessor) Started() {
	// initialize a new writer.
	p.writer = csv.NewWriter(os.Stdout)
}

func (p *CsvProcessor) ProcessConfig(c *Config) {
	p.currConfig = c
}

func (p *CsvProcessor) ProcessResult(r *Result) {
	fields := make([]string, 5)
	if r.Error == nil {
		fields[0] = "OK"
	} else {
		fields[0] = "FAIL"
	}

	fields[1] = p.currConfig.Name
	fields[2] = r.Monitor.Name
	fields[3] = r.Monitor.Url
	fields[4] = r.Latency.String()

	p.writer.Write(fields)
	p.writer.Flush()
}

func (p *CsvProcessor) Finished() {
	p.writer.Flush()
}
