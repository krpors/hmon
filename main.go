package main

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// the version string for hmon.
const VERSION string = "1.0.4"

// cmdline flag variables
var (
	flagConf         = flag.String("conf", "", "Single configuration file. This param takes precedence over -confdir.")
	flagConfdir      = flag.String("confdir", ".", "Directory with configurations of *_hmon.xml files.")
	flagFiledir      = flag.String("filedir", ".", "Base directory to search for request files. If ommited, the current working directory is used.")
	flagValidateOnly = flag.Bool("validate", false, "When specified, only validate the configuration file(s), but don't run the monitors.")
	flagOutput       = flag.String("output", "", "Output file or directory. If empty, output will be done to stdout only.")
	flagFormat       = flag.String("format", "", "Output format ('csv', 'json', 'pandora'). Only suitable in combination with -outfile .")
	flagVersion      = flag.Bool("version", false, "Prints out version number and exits (discards other flags).")
	flagSequential   = flag.Bool("sequential", false, "When set, execute monitors in sequential order (not recommended for speed).")
	flagCombine      = flag.Bool("combine", false, "If set, combine all monitors from all configurations to run, instead of per configuration.")
	flagVerbose      = flag.Bool("verbose", false, "Set verbose output. Helpful to see input and output being sent and received.")
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

	// TODO: check for uniqueness of monitor NAMES, emit warning if not unique.

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
func writeDefault(filename string, r *[]ConfigurationResult) error {
	// TODO this
	fmt.Println("Writing default")
	return nil
}

// Writes the slice of results to the given filename as Json.
// Any error will exit the program with exitcode 1.
func writeJson(filename string, r *[]ConfigurationResult) error {
	b, err := json.MarshalIndent(r, "  ", "  ")
	if err != nil {
		return fmt.Errorf("Error marshaling json: %s", err)
	}

	err = ioutil.WriteFile(filename, b, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write to file `%s': %s\n", filename, err)
	}

	return nil
}

// Writes the slice of results to the given filename as CSV. If any error
// occurs, exit with code 1.
func writeCsv(filename string, results *[]ConfigurationResult) error {
	f, err := os.Create(filename)

	if err != nil {
		return fmt.Errorf("Unable to open file for writing `%s': %s\n", filename, err)
	}

	w := csv.NewWriter(f)

	for _, r := range *results {
		for _, res := range r.Results {
			var status string = "FAIL"
			if res.Error == nil {
				status = "OK"
			}

			record := []string{
				status,
				res.Monitor.Name,
				res.Monitor.Url,
				strconv.FormatInt(res.Latency, 10),
			}
			w.Write(record)
		}
	}
	w.Flush()

	return nil
}

// Writes all results to Pandora Agent interpretable XML files.
func writePandoraAgents(outdir string, results *[]ConfigurationResult) error {
	fmt.Printf("Writing %d configuration results to %s\n", len(*results), outdir)

	for _, result := range *results {

		pfmsAgent := PfmsAgent{}
		pfmsAgent.AgentName = result.ConfigurationName
		pfmsAgent.Timestamp = CreatePfmsTimstamp()
		pfmsAgent.GroupName = "Web Services" // ugh, currently hardcoded. Ah well, we'll fix that later.

		for _, actualResult := range result.Results {
			module := PfmsModule{}
			module.Name = actualResult.Monitor.Name
			module.Type = "generic_data"
			module.Description = actualResult.Monitor.Description
			if actualResult.Error != nil {
				module.Data = "-1"
			} else {
				module.Data = strconv.FormatInt(actualResult.Latency, 10)
			}

			pfmsAgent.Modules = append(pfmsAgent.Modules, module)
		}

		// write agent to file...
		xmlBytes, err := xml.MarshalIndent(pfmsAgent, " ", "   ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not marshal PFMS data to bytes: %s\n", err)
			os.Exit(1)
		}
		outputFile := fmt.Sprintf("%s-%d.xml", result.ConfigurationName, time.Now().Unix())
		outputPath := path.Join(outdir, outputFile)
		err = ioutil.WriteFile(outputPath, xmlBytes, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not write to file: %s\n", err)
			os.Exit(1)
		}
	}

	return nil
}

// Returns a timestamp in the format of yyyy/MM/dd HH:mm:ss, because a Pandora Agent expects it like that.
// Hooray for no ISO 8601!
func CreatePfmsTimstamp() string {
	currenttime := time.Now()
	return currenttime.Format("2006/01/02 15:04:05")
}

type PfmsAgent struct {
	XMLName   struct{}     `xml:"agent_data"`
	AgentName string       `xml:"agent_name,attr"`
	GroupName string       `xml:"group,attr"`
	Timestamp string       `xml:"timestamp,omitempty"` //TODO: Timestamp indicating when the XML file was generated (YYYY/MM/DD HH:MM:SS).
	Modules   []PfmsModule `xml:"module"`
}

type PfmsModule struct {
	Name        string `xml:"name"`
	Type        string `xml:"type"`
	Description string `xml:"description,omitempty"`
	Data        string `xml:"data"`
}

// When the verbose flag is supplied, each monitor is getting this as a callback
// to output TODO: dox
func verboseCallback(monitor *Monitor, input, output []byte) {
	fmt.Printf("=================\n")
	fmt.Printf("Monitor '%s'\n", monitor.Name)
	fmt.Printf("INPUT:\n%s\n", string(input))
	fmt.Printf("OUTPUT:\n%s\n", string(output))
	fmt.Printf("=================\n")
}

// Run the given monitors in sequential order, and return the results.
func runSequential(filedir string, config Config, verbose bool) ConfigurationResult {
	// receiver channel
	ch := make(chan Result)

	results := ConfigurationResult{}
	results.ConfigurationName = config.Name

	for _, mon := range config.Monitors {
		if verbose {
			mon.Callback = verboseCallback
		}
		go mon.Run(filedir, ch)
		// immediately receive from the channel
		result := <-ch
		results.Results = append(results.Results, result)
		fmt.Printf("%s\n", result)
	}

	return results
}

func runParallel(filedir string, config Config, verbose bool) ConfigurationResult {
	// receiver channel
	ch := make(chan Result, len(config.Monitors))

	results := ConfigurationResult{}
	results.ConfigurationName = config.Name

	for _, mon := range config.Monitors {
		// fire all goroutines first
		if verbose {
			mon.Callback = verboseCallback
		}
		go mon.Run(filedir, ch)
	}

	// then receive from the channel
	for _ = range config.Monitors {
		result := <-ch
		results.Results = append(results.Results, result)
		fmt.Printf("%s\n", result)
	}

	return results
}

// Prints a short execution summary using all the results gathered.
func printExecutionSummary(configResults []ConfigurationResult) {
	var total int = 0
	var countOk int = 0
	var countFail int = 0

	for _, cr := range configResults {
		for _, res := range cr.Results {
			total++
			if res.Error == nil {
				countOk++
			} else {
				countFail++
			}
		}
	}

	fmt.Printf("\nExecution summary:\n")
	fmt.Printf("Monitors:  %d\n", total)
	fmt.Printf("Successes: %d\n", countOk)
	fmt.Printf("Failures:  %d\n", countFail)

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
		fmt.Fprintf(os.Stderr, "FLAGS (with defaults):\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// If version is requested, report that and then exit normally.
	if *flagVersion {
		fmt.Fprintf(os.Stderr, "hmon version %s\n", VERSION)
		os.Exit(0)
	}

	var writeFunc func(string, *[]ConfigurationResult) error
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
	case "pandora":
		writeFunc = writePandoraAgents
		break
	default:
		// unknown output format. Bail out
		fmt.Printf("Unknown output format: %s\n", *flagFormat)
		os.Exit(1)
	}

	// check if the output format is pandora, and if we are trying to combine the monitors.
	// if so, bail out since the combination does not work out well... FOR NOW XXX XXX XXX check this!
	if *flagFormat == "pandora" && *flagCombine {
		fmt.Fprintf(os.Stderr, "Using output type 'pandora' with -combine will not yield correct results.\n")
		os.Exit(1)
	}

	// Emit a warning that no output file or directory is specified. Only tell the user
	// this when a different format is specified.
	if *flagFormat != "" && strings.TrimSpace(*flagOutput) == "" {
		fmt.Printf("Warning: no explicit output file or directory specified. No file(s) will be created!\n")
	}

	var configurations []Config
	var err error

	// Check if we should read a single configuration, or a configuration directory.
	if *flagConf != "" {
		c, err := ReadConfig(*flagConf)
		if err != nil {
			fmt.Printf("Unable to parse single configuration file `%s': %s\n", *flagConf, err)
			os.Exit(1)
		}
		// just append the parsed config to the slice. It should now be 1 in length, only.
		configurations = append(configurations, c)
	} else {
		// First, find the configurations from the flagConfdir. Bail if anything fails.
		configurations, err = FindConfigs(*flagConfdir)
		if err != nil {
			fmt.Printf("Unable to find/parse configuration files. Nested error is: %s\n", err)
			os.Exit(1)
		}
	}

	validateConfigurations(&configurations)

	_, err = os.Open(*flagFiledir)
	if err != nil {
		fmt.Printf("Failed to open request directory. Nested error is: %s\n", err)
		os.Exit(1)
	}

	var configResults []ConfigurationResult

	// Are we supposed to run the monitors per configuration?
	if !*flagCombine {
		for _, c := range configurations {
			fmt.Printf("Processing configuration `%s' with %d monitors\n", c.Name, len(c.Monitors))

			// should we run in parallel?
			var cr ConfigurationResult
			if !*flagSequential {
				cr = runParallel(*flagFiledir, c, *flagVerbose)
			} else {
				// or sequential.
				cr = runSequential(*flagFiledir, c, *flagVerbose)
			}
			configResults = append(configResults, cr)
		}
	} else {
		// or are we combining them all in one large slice of Monitors first?
		allMonitors := make([]Monitor, 0)
		for _, c := range configurations {
			allMonitors = append(allMonitors, c.Monitors...)
		}

		combinedConfig := Config{}
		combinedConfig.Name = "Combined hmon configuration"
		combinedConfig.Monitors = allMonitors

		fmt.Printf("Running %d monitors from %d configurations combined.\n", len(allMonitors), len(configurations))

		// should we run in parallel?
		var cr ConfigurationResult
		if !*flagSequential {
			cr = runParallel(*flagFiledir, combinedConfig, *flagVerbose)
		} else {
			// or sequential.
			cr = runSequential(*flagFiledir, combinedConfig, *flagVerbose)
		}
		configResults = append(configResults, cr)

	}

	// print execution summary with totals, amount failed, amount ok, etc.
	printExecutionSummary(configResults)

	fmt.Println()

	if strings.TrimSpace(*flagOutput) != "" {
		// sanity nil check.
		if writeFunc != nil {
			err := writeFunc(*flagOutput, &configResults)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
}
