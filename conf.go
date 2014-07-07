package main

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
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
const TimeoutDefault int = 60

/*
 * ===============================================================================
 * Error type used for reporting validation errors on the configuration file.
 * ===============================================================================
 */

// ValidationError is used to return validation errors when validation fails. It
// simply contains a list of errors as a string slice.
type ValidationError struct {
	// Just an error list with all validation errors.
	ErrorList []string
}

// Add simply appends the string to the stack.
func (e *ValidationError) Add(s string) {
	e.ErrorList = append(e.ErrorList, s)
}

// Returns a summary of the errors as a one-liner.
func (e ValidationError) Error() string {
	plural := "error"
	if len(e.ErrorList) > 1 {
		plural = "errors"
	}
	return fmt.Sprintf("%d config validation %s found", len(e.ErrorList), plural)
}

// ResultError is a typ which implements the error and json.Marshaler interface. It's a sort of wrapper
// around the error type, so an error can be marshaled to JSON (i.e. the error description).
type ResultError struct {
	Err error // the encapsulated error
}

// Returns the description of the error.
func (r ResultError) Error() string {
	return fmt.Sprintf("%v", r.Err)
}

// MarshalJSON implements the interface json.Marshaler. Empty descriptions result in a null JSON object.
// Non empty descriptions are just simply marshaled.
func (r ResultError) MarshalJSON() ([]byte, error) {
	if r.Err == nil {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", r.Err)), nil
}

/*
 * ===============================================================================
 * Main configuration structure. Parsed through reflection by the toml proc.
 * ===============================================================================
 */

// Config is the root configuration node, contains zero or more Monitor structs.
type Config struct {
	// The original filename (basename)
	FileName string

	Name    string
	Monitor map[string]Monitor
}

// Validate runs a validation over the parsed configuration file. The returned
// error is of type ValidationError.
func (c *Config) Validate(basePath string) error {
	verr := ValidationError{}

	if strings.TrimSpace(c.Name) == "" {
		verr.Add("Configuration file must have a 'name'")
	}

	for monitorName, monitor := range c.Monitor {
		if monitor.Name == "" {
			verr.Add(fmt.Sprintf("monitor '%s' must have a 'name' attribute", monitorName))
		}
		if monitor.URL == "" {
			verr.Add(fmt.Sprintf("monitor '%s': must have a 'url' attribute", monitorName))
		} else {
			_, err := url.ParseRequestURI(monitor.URL)
			if err != nil {
				verr.Add(fmt.Sprintf("monitor '%s': malformed url (%s)", monitorName, err))
			}
		}

		// validate headers, if applicable
		for _, header := range monitor.Headers {
			err := header.Validate()
			if err != nil {
				verr.Add(fmt.Sprintf("monitor '%s': malformed header spec: %s", monitorName, err))
			}
		}

		// try to open the file which is to be sent
		if monitor.File != "" {
			f := path.Join(basePath, monitor.File)
			_, err := os.Stat(f)
			if err != nil {
				verr.Add(fmt.Sprintf("monitor '%s': unable to use HTTP POST data: %s", monitorName, err))
			}
		}

		for _, assertion := range monitor.Assertions {
			_, err := regexp.Compile(assertion)
			if err != nil {
				verr.Add(fmt.Sprintf("monitor '%s': assertion '%s' has an invalid regex: %s", monitorName, assertion, err))
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

// Monitor node with its children. All slices can be zero or more,
// technically, but logically some kind of validation can be done using
// Validate().
type Monitor struct {
	Name        string
	Description string
	URL         string
	File        string
	Timeout     int
	Headers     []Header
	Assertions  []string
	Callback    func(*Monitor, []byte, []byte) `json:"-"` // callback function to check input/output
}

// notifyCallback will report the input and output when hmon is run in verbose mode.
func (m *Monitor) notifyCallback(input, output []byte) {
	if m.Callback != nil {
		m.Callback(m, input, output)
	}
}

// Run runs a check for the given Monitor. There are a few things done in this function.
// If the given input file is empty (i.e. none), a http GET is issued to the given URL.
// If a file is given though, this will become a http POST, with the post-data being the
// file's contents. If there are any assertions configured, all the assertions are used
// to test the content. If none are configured, it will just be a sort of 'ping-check',
// i.e. checking if a connection could be made to the URL.
func (m Monitor) Run(baseDir string, c chan Result) {
	client := http.Client{}

	var requestBody []byte
	var req *http.Request
	var err error

	if m.File == "" {
		req, err = http.NewRequest("GET", m.URL, nil)
	} else {
		requestBody, err = ioutil.ReadFile(path.Join(baseDir, m.File))
		if err != nil {
			m.notifyCallback(requestBody, nil)
			c <- Result{m, 0, ResultError{err}}
			return
		}
		req, err = http.NewRequest("POST", m.URL, bytes.NewReader(requestBody))
	}

	if err != nil {
		m.notifyCallback(requestBody, nil)
		c <- Result{m, 0, ResultError{err}}
		return
	}

	// add all optional headers. This uses the GetName() and GetValue on our Header
	// type. By this time, the validator should have validated the headers in the
	// configuration, so correct headers are sent.
	for _, header := range m.Headers {
		req.Header.Set(header.GetName(), header.GetValue())
	}

	// start measuring time from this point:
	tstart := time.Now()

	// This block enables us to timeout the HTTP call.
	type response struct {
		Resp *http.Response
		Err  error
	}
	timeoutChan := make(chan response, 1)

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
		timeout = time.Duration(TimeoutDefault) * time.Second
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
	return fmt.Sprintf("Monitor '%s' to URL %s, %d headers, %d assertions", m.Name, m.URL, len(m.Headers), len(m.Assertions))
}

// Header is a string type with a HTTP header in the form of "Header: value". The type defines
// two methods to extract the name and the value from it.
type Header string

// GetName finds the name of the header by splitting the header string on the colon character.
func (h Header) GetName() string {
	idx := strings.Index(string(h), ":")
	if idx > 0 {
		return string(h)[0:idx]
	}

	return ""
}

// GetValue finds the value of the header by splitting the header string on the colon character.
// Returns an empty string when the length
func (h Header) GetValue() string {
	idx := strings.Index(string(h), ":")
	if idx > 0 {
		return strings.Trim(string(h)[idx+1:], " ")
	}

	return ""
}

// Validate slightly validates if a header is correct. I'm not implementing the full spec here though.
func (h Header) Validate() error {
	idx := strings.Index(string(h), ":")
	if idx < 0 {
		return fmt.Errorf("invalid header '%s'", string(h))
	}

	hname := string(h)[0:idx]
	// lol, not sure if this is ok, but I'm currently too lazy to read the http header field name
	// spec things. For now this is ok.
	if strings.ContainsAny(hname, " ,.!=+@#$%^&*") {
		return fmt.Errorf("invalid header name '%s'", hname)
	}

	return nil
}

/*
 * ===============================================================================
 * Functions not belonging to types
 * ===============================================================================
 */

// ReadConfig reads a single toml configuration file name. Returns a Config struct if OK,
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

	c := Config{}
	c.FileName = finfo.Name()
	_, err = toml.DecodeFile(file, &c)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse file `%s': %s", file, err)
	}

	return c, nil
}

// FindConfigs find all toml configuration files using a base directory. A slice of Config
// are returned. If the slice length is zero, and the error is non-nil, no configurations are found.
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
			if strings.HasSuffix(fi.Name(), "_hmon.toml") {
				fullFile := path.Join(baseDir, fi.Name())

				c := Config{}
				c.FileName = fi.Name()

				_, err := toml.DecodeFile(fullFile, &c)
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

/*
 * ===============================================================================
 * Misc util structs.
 * ===============================================================================
 */

// ConfigurationResult encapsulates a given configuration with the results
// for that configuration.
type ConfigurationResult struct {
	ConfigurationName string   // the identifiable name of the configuration
	Results           []Result // the results for this configuration
}

// Result encapsulates information about a Monitor and its invocation result.
type Result struct {
	Monitor Monitor // the monitor which may or may not have failed.
	Latency int64   // The latency of the call i.e. how long did it take (in ms)
	Error   error   // An error, describing the possible failure. If nil, it's ok.
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
