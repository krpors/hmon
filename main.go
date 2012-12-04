package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// the version string for hmon.
const VERSION string = "0.1"

// cmdline flag variables
var (
	flagConfdir      = flag.String("confdir", ".", "Directory with configurations of *_hmon.xml files.")
	flagFiledir      = flag.String("filedir", ".", "Base directory to search for request files. If ommited, the current working directory is used.")
	flagValidateOnly = flag.Bool("validate", false, "When specified, only validate the configuration file(s), but don't run the monitors.")
	flagOutfile      = flag.String("outfile", "", "Output to given file. If empty, output will be done to stdout only.")
	flagFormat       = flag.String("format", "", "Output format ('csv', 'html', 'json'). Only suitable in combination with -outfile .")
	flagVersion      = flag.Bool("version", false, "Prints out version number and exits (discards other flags).")
	flagSequential   = flag.Bool("sequential", false, "When set, execute monitors in sequential order (not recommended for speed).")
)

// Validates all configurations in the slice. For every failed validation,
// print it out to stdout. If any failures occured, simply bail out.
func validateConfigurations(configurations *[]Config) {
	if len(*configurations) == 0 {
		fmt.Printf("No configurations found were found in `%s'\n", *flagConfdir)
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
	if *flagValidateOnly {
		// if so, no point in continuing. Exit code 0 to indicate an a-okay.
		fmt.Println("ok")
		os.Exit(0)
	}
}

func writeDefault(filename string, r *[]Result) {
	fmt.Println("Writing default")
}

func writeJson(filename string, r *[]Result) {
	// TODO: print to json to file.
	b, err := json.MarshalIndent(r, "\t", "\t")
	if err != nil {
		fmt.Printf("Error marshaling json: %s", err)
		os.Exit(1)
		return // shouldn't occur
	}

	err = ioutil.WriteFile(filename, b, 644)
	if err != nil {
		fmt.Printf("Unable to write to file `%s': %s\n", filename, err)
		os.Exit(1)
	}
}

func writeCsv(filename string, r *[]Result) {
	// TODO: print to csv to file
	fmt.Println("writing csv")
}

func main() {
	// cmdline usage function. Prints out to stderr of course.
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "hmon version %s\n\n", VERSION)
		fmt.Fprintf(os.Stderr, "A simplistic host monitor using content assertions. This tool connects to\n")
		fmt.Fprintf(os.Stderr, "configured http serving hosts, issues a request and checks the content using\n")
		fmt.Fprintf(os.Stderr, "regular expression 'assertions'.\n\n")
		fmt.Fprintf(os.Stderr, "Normal output is done to the standard output, and using the flag -outfile\n")
		fmt.Fprintf(os.Stderr, "combined with -outtype the results can be written to different file formats.\n\n")
		fmt.Fprintf(os.Stderr, "For more information, check the GitHub page at http://github.com/krpors/hmon.\n\n")
		fmt.Fprintf(os.Stderr, "FLAGS:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// If version is requested, report that and then exit normally.
	if *flagVersion {
		fmt.Fprintf(os.Stderr, "hmon version %s\n", VERSION)
		os.Exit(0)
	}

	var writeFunc func(string, *[]Result)
	// determine type of format
	switch *flagFormat {
	case "":
		writeFunc = writeDefault
		break
	case "json":
		writeFunc = writeJson
		break
	case "csv":
		writeFunc = writeCsv
		break
	default:
		// unknown output format. Bail out
		fmt.Printf("Unknown output format: %s\n", *flagFormat)
		os.Exit(1)
	}

	// First, find the configurations from the flagConfdir. Bail if anything fails.
	configurations, err := FindConfigs(*flagConfdir)
	if err != nil {
		fmt.Printf("Unable to find/parse configuration files. Nested error is: %s\n", err)
		os.Exit(1)
	}

	validateConfigurations(&configurations)

	_, err = os.Open(*flagFiledir)
	if err != nil {
		fmt.Printf("Failed to open request directory. Nested error is: %s\n", err)
		os.Exit(1)
	}

	results := make([]Result, 0)

	for _, c := range configurations {
		fmt.Printf("Processing configuration `%s' with %d monitors\n", c.Name, len(c.Monitors))

		// receiver channel
		ch := make(chan Result, len(c.Monitors))

		for i := range c.Monitors {
			if *flagSequential {
				c.Monitors[i].Run(*flagFiledir, ch)
				result := <-ch
				fmt.Printf("%s\n", result)
				results = append(results, result)
			} else {
				go c.Monitors[i].Run(*flagFiledir, ch)
			}
		}

		// read from the channel until all monitors have sent their response
		if !*flagSequential {
			for _ = range c.Monitors {
				result := <-ch
				fmt.Printf("%s\n", result)
				results = append(results, result)
			}
		}
	}

	var countOk int = 0
	var countFail int = 0
	for _, r := range results {
		if r.Error == nil {
			countOk++
		} else {
			countFail++
		}
	}

	fmt.Printf("\nExecuted %d monitors with %d successes and %d failures.\n", len(results), countOk, countFail)

	// TODO: check output file and type, write it to file using the printXxxx functions.
	if strings.TrimSpace(*flagOutfile) != "" {
		// sanity nil check.
		if writeFunc != nil {
			writeFunc(*flagOutfile, &results)
		}
	}
}
