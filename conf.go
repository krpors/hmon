package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
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
