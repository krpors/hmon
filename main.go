package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// the version string for hmon.
const VERSION string = "1.0.1"

// cmdline flag variables
var (
	flagConfdir      = flag.String("confdir", ".", "Directory with configurations of *_hmon.xml files.")
	flagFiledir      = flag.String("filedir", ".", "Base directory to search for request files. If ommited, the current working directory is used.")
	flagValidateOnly = flag.Bool("validate", false, "When specified, only validate the configuration file(s), but don't run the monitors.")
	flagOutfile      = flag.String("outfile", "", "Output to given file. If empty, output will be done to stdout only.")
	flagFormat       = flag.String("format", "", "Output format ('csv', 'json'). Only suitable in combination with -outfile .")
	flagVersion      = flag.Bool("version", false, "Prints out version number and exits (discards other flags).")
	flagSequential   = flag.Bool("sequential", false, "When set, execute monitors in sequential order (not recommended for speed).")
	flagCombine      = flag.Bool("combine", false, "If set, combine all monitors from all configurations to run, instead of per configuration.")
)

// Validates all configurations in the slice. For every failed validation,
// print it out to stdout. If any failures occured, simply bail out with exitcode 1.
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

// Writes a non-specialized format to the given filename.
func writeDefault(filename string, r *[]Result) {
	// TODO this
	fmt.Println("Writing default")
}

// Writes the slice of results to the given filename as Json.
// Any error will exit the program with exitcode 1.
func writeJson(filename string, r *[]Result) {
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

// Writes the slice of results to the given filename as CSV. If any error
// occurs, exit with code 1.
func writeCsv(filename string, r *[]Result) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Unable to open file for writing `%s': %s\n", filename, err)
		os.Exit(1)
	}

	w := csv.NewWriter(f)

	for _, e := range *r {
		var status string = "FAIL"
		if e.Error == nil {
			status = "OK"
		}

		record := []string{
			status,
			e.Monitor.Name,
			e.Monitor.Url,
			strconv.FormatInt(e.Latency, 10),
		}
		w.Write(record)
	}
	w.Flush()
}

// Run the given monitors in sequential order, and return the results.
func runSequential(filedir string, m []Monitor) []Result {
	// receiver channel
	ch := make(chan Result)

	results := make([]Result, 0)

	for i := range m {
		go m[i].Run(filedir, ch)
		// immediately receive from the channel
		result := <-ch
		results = append(results, result)
		fmt.Printf("%s\n", result)
	}

	return results
}

// Run the given monitors in parallel order, and return the results.
func runParallel(filedir string, m []Monitor) []Result {
	// receiver channel
	ch := make(chan Result, len(m))

	results := make([]Result, 0)

	for i := range m {
		// fire all goroutines first
		go m[i].Run(filedir, ch)
	}

	// then receive from the channel
	for _ = range m {
		result := <-ch
		results = append(results, result)
		fmt.Printf("%s\n", result)
	}

	return results
}

// Entry point of this program.
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

	// Are we supposed to run the monitors per configuration?
	if !*flagCombine {
		for _, c := range configurations {
			fmt.Printf("Processing configuration `%s' with %d monitors\n", c.Name, len(c.Monitors))

			// should we run in parallel?
			if !*flagSequential {
				presults := runParallel(*flagFiledir, c.Monitors)
				results = append(results, presults...)
			} else {
				// or sequential.
				sresults := runSequential(*flagFiledir, c.Monitors)
				results = append(results, sresults...)
			}
		}
	} else {
		// or are we combining them all in one large slice of Monitors first?
		allMonitors := make([]Monitor, 0)
		for _, c := range configurations {
			allMonitors = append(allMonitors, c.Monitors...)
		}

		fmt.Printf("Running %d monitors from %d configurations combined.\n", len(allMonitors), len(configurations))

		// should we run in parallel?
		if !*flagSequential {
			presults := runParallel(*flagFiledir, allMonitors)
			results = append(results, presults...)
		} else {
			// or sequential.
			sresults := runSequential(*flagFiledir, allMonitors)
			results = append(results, sresults...)
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
