package main

import (
	"bytes"
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

// Default timeout in seconds
const TIMEOUT_DEFAULT int = 60

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

// This type implements the error and json.Marshaler interface. It's a sort of wrapper
// around the error type, so an error can be marshaled to JSON (i.e. the error description).
type ResultError struct {
	Err error // the encapsulated error
}

// Returns the description of the error.
func (r ResultError) Error() string {
	return fmt.Sprintf("%v", r.Err)
}

// The function from interface json.Marshaler. Empty descriptions result in a null JSON object.
// Non empty descriptions are just simply marshaled.
func (r ResultError) MarshalJSON() ([]byte, error) {
	if r.Err == nil {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", r.Err)), nil
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
	Name        string                         `xml:"name,attr"`
	Description string                         `xml:"desc,attr"`
	Url         string                         `xml:"url"`
	File        string                         `xml:"file"`
	Timeout     int                            `xml:"timeout"`
	Headers     []Header                       `xml:"headers>header"`
	Assertions  []string                       `xml:"assertions>assertion"`
	Callback    func(*Monitor, []byte, []byte) `json:"-"` // callback function to check input/output
}

// Extra HTTP headers to send.
type Header struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (m *Monitor) notifyCallback(input, output []byte) {
	if m.Callback != nil {
		m.Callback(m, input, output)
	}
}

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

	var requestBody []byte
	var req *http.Request
	var err error

	if m.File == "" {
		req, err = http.NewRequest("GET", m.Url, nil)
	} else {
		requestBody, err := ioutil.ReadFile(path.Join(baseDir, m.File))
		if err != nil {
			m.notifyCallback(requestBody, nil)
			c <- Result{m, 0, ResultError{err}}
			return
		}
		req, err = http.NewRequest("POST", m.Url, bytes.NewReader(requestBody))
	}

	if err != nil {
		m.notifyCallback(requestBody, nil)
		c <- Result{m, 0, ResultError{err}}
		return
	}

	// add all optional headers:
	for i := range m.Headers {
		req.Header.Add(m.Headers[i].Name, m.Headers[i].Value)
	}

	// start measuring time from this point:
	tstart := time.Now()

	// This block enables us to timeout the HTTP call.
	type response struct {
		Resp *http.Response
		Err  error
	}
	timeoutChan := make(chan response)

	// run the Do in a goroutine, and write the response to the timeout channel.
	go func() {
		resp, err := client.Do(req)
		timeoutChan <- response{resp, err}
	}()

	var theResponse response

	// read from the channel, or until timeout
	var timeout time.Duration
	if m.Timeout <= 0 {
		// if timeout is smaller/eq zero, use default timeout
		timeout = time.Duration(TIMEOUT_DEFAULT) * time.Second
	} else {
		timeout = time.Duration(int64(m.Timeout)) * time.Millisecond
	}

	select {
	case <-time.After(timeout):
		m.notifyCallback(requestBody, nil)
		c <- Result{m, 0, ResultError{fmt.Errorf("timeout after %d ms", timeout/time.Millisecond)}}
		return
	case theResponse = <-timeoutChan:
		// OKAY! We got a response.
	}

	// check any errors in the response itself
	if theResponse.Err != nil {
		m.notifyCallback(requestBody, nil)
		c <- Result{m, 0, ResultError{theResponse.Err}}
		return
	}

	// we got no errors now, i.e. we got an actual response body. Defer closing it,
	// and read from it so we can process it further.
	defer theResponse.Resp.Body.Close()
	responseContents, err := ioutil.ReadAll(theResponse.Resp.Body)

	// whether the response validates against the assertions.
	// When no assertions are given, just check if the site/host is up.
	for i := range m.Assertions {
		// at this point, compilation of the regular expression must succeed,
		// since we already executed a Validate() on the configuration itself.
		// To make things sure, we do a MustCompile though.
		rex := regexp.MustCompile(m.Assertions[i])
		found := rex.Find(responseContents)
		if found == nil {
			millis := int64(time.Now().Sub(tstart) / time.Millisecond)
			m.notifyCallback(requestBody, responseContents)
			c <- Result{m, millis, ResultError{fmt.Errorf("assertion failed for regex `%s'", m.Assertions[i])}}
			return
		}
	}

	// passed all tests, return true to the channel
	millis := int64(time.Now().Sub(tstart) / time.Millisecond)

	m.notifyCallback(requestBody, responseContents)
	c <- Result{m, millis, nil}
}

// Returns the monitor as a string.
func (m Monitor) String() string {
	return fmt.Sprintf("%s (%s), %d headers, %d assertions", m.Name, m.Url, len(m.Headers), len(m.Assertions))
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

// Reads a single configuration file name. Returns a Config struct if OK,
// or an error if anything has failed.
func ReadConfig(file string) (Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return Config{}, err
	}

	finfo, err := f.Stat()

	if finfo.IsDir() {
		return Config{}, fmt.Errorf("`%s' is not a regular file", file)
	}

	contents, err := ioutil.ReadFile(file)
	if err != nil {
	}

	c := Config{}
	c.FileName = finfo.Name()
	err = xml.Unmarshal(contents, &c)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse file `%s': %s", file, err)
	}

	return c, nil
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
			if strings.HasSuffix(fi.Name(), "_hmon.xml") {
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
	Monitor *Monitor // the monitor which may or may not have failed.
	Latency int64    // The latency of the call i.e. how long did it take (in ms)
	Error   error    // An error, describing the possible failure. If nil, it's ok.
}

// Returns the result as a string for some easy-peasy debuggin'.
func (r Result) String() string {
	if r.Error == nil {
		return fmt.Sprintf("ok    %s (%d ms)", r.Monitor.Name, r.Latency)
	}

	if r.Latency > 0 {
		return fmt.Sprintf("FAIL  %s: %s (%d ms)", r.Monitor.Name, r.Error, r.Latency)
	}

	return fmt.Sprintf("FAIL  %s: %s", r.Monitor.Name, r.Error)
}
