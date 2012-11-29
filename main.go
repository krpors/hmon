package main

import (
	"flag"
	"fmt"
	"os"
)

// the version string for hmon.
const VERSION string = "0.1"

// cmdline flag variables
var (
	flagConfdir      = flag.String("confdir", ".", "Directory with configurations of *_hmon.xml files.")
	flagFiledir      = flag.String("filedir", ".", "Base directory to search for request files. If ommited, the current working directory is used.")
	flagValidateOnly = flag.Bool("validate", false, "When specified, only validate the configuration file(s), but don't run the monitors.")
	flagOutput       = flag.String("output", "default", "Output format ('default')")
	flagVersion      = flag.Bool("version", false, "Prints out version number and exits (discards other flags)")
	flagSequential   = flag.Bool("sequential", false, "When set, execute monitors in sequential order (not recommended for speed)")
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

func main() {
	// cmdline usage function. Prints out to stderr of course.
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "hmon version %s\n\n", VERSION)
		fmt.Fprintf(os.Stderr, "A simplistic host monitor using content assertions. This tool connects to\n")
		fmt.Fprintf(os.Stderr, "configured http serving hosts, issues a request and checks the content using\n")
		fmt.Fprintf(os.Stderr, "regular expression 'assertions'. For more information, check the GitHub page\n")
		fmt.Fprintf(os.Stderr, "at http://github.com/krpors/hmon.\n\n")
		fmt.Fprintf(os.Stderr, "FLAGS:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// If version is requested, report that and then exit normally.
	if *flagVersion {
		fmt.Fprintf(os.Stderr, "hmon version %s\n", VERSION)
		os.Exit(0)
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

	results := make([]Result, 1)

	// the result processor to use, depending in the flag "output"
	var processor ResultProcessor
	// determine processor here
	switch *flagOutput {
	case "default":
		processor = DefaultProcessor{}
	case "html":
		processor = DefaultProcessor{}
	case "csv":
		processor = DefaultProcessor{}
	default:
		processor = DefaultProcessor{}
	}

	processor.Started()
	for _, c := range configurations {
		processor.ProcessConfig(&c)

		// receiver channel
		ch := make(chan Result, len(c.Monitors))

		for i := range c.Monitors {
			if *flagSequential {
				c.Monitors[i].Run(*flagFiledir, ch)
				result := <-ch
				processor.ProcessResult(&result)
			} else {
				go c.Monitors[i].Run(*flagFiledir, ch)
			}
		}

		// read from the channel until all monitors have sent their response
		if !*flagSequential {
			for _ = range c.Monitors {
				result := <-ch
				processor.ProcessResult(&result)
			}
		}
	}
	processor.Finished()

	_ = results
}
