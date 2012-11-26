package main

import (
	"fmt"
	"net/url"
	"regexp"
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
	Monitor []Monitor `xml:"monitor"`
}

// Monitor node with its children. All slices can be zero or more,
// technically, but logically some kind of validation can be done using
// Validate().
type Monitor struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"desc,attr"`
	Url         string   `xml:"url"`
	File        string   `xml:"req"`
	Timeout     int      `xml:"timeout"`
	Headers     []Header `xml:"headers>header"`
	Assertions  []string `xml:"assertions>assertion"`
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

	for monidx, mon := range c.Monitor {
		if mon.Url == "" {
			verr.Add(fmt.Sprintf("monitor[%d]: empty url", monidx))
		} else {
			_, err := url.ParseRequestURI(mon.Url)
			if err != nil {
				verr.Add(fmt.Sprintf("monitor[%d]: malformed url: %s", monidx, err))
			}
		}

		for assidx, assert := range mon.Assertions {
			_, err := regexp.Compile(assert)
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
