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
const VERSION string = "1.1.0"

// cmdline flag variables
var (
	flagConf         = flag.String("conf", "", "Single configuration file. This param takes precedence over -confdir.")
	flagConfdir      = flag.String("confdir", ".", "Directory with configurations of *_hmon.xml files.")
	flagFiledir      = flag.String("filedir", ".", "Base directory to search for request files. If ommited, the current working directory is used.")
	flagValidateOnly = flag.Bool("validate", false, "When specified, only validate the configuration file(s), but don't run the monitors.")
	flagOutput       = flag.String("output", "", "Output file or directory. If empty, output will be done to stdout only.")
	flagFormat       = flag.String("format", "", "Output format ('csv', 'json', 'pandora'). Only suitable in combination with -output.")
	flagVersion      = flag.Bool("version", false, "Prints out version number and exits (discards other flags).")
	flagSequential   = flag.Bool("sequential", false, "When set, execute monitors in sequential order (not recommended for speed).")
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
	success := true
	var totalerrs int8

	// first, check for failures in monitors inside a each configuration
	for _, c := range *configurations {
		err := c.Validate(*flagFiledir)
		if err != nil {
			// we got validation errors.
			verr := err.(ValidationError)
			fmt.Printf("%s: %s\n", c.FileName, verr)
			for i := range verr.ErrorList {
				fmt.Printf("  %s\n", verr.ErrorList[i])
				totalerrs++
			}

			success = false
			fmt.Println()
		}
	}

	// TODO: check for uniqueness of monitor NAMES, emit warning if not unique.
	mapConfigNames := make(map[string]string) // map is configname:filename

	// secondly, check for the uniqueness of the hmonconfig names (attribute in the root node)
	for _, c := range *configurations {
		filename, foundInMap := mapConfigNames[c.Name]
		if foundInMap {
			fmt.Printf("%s: hmonconfig name '%s' is already defined in file '%s'\n", c.FileName, c.Name, filename)
			success = false
			totalerrs++
		} else {
			mapConfigNames[c.Name] = c.FileName
		}
	}

	if !success {
		plural := "errors"
		if totalerrs <= 1 {
			plural = "error"
		}
		fmt.Printf("\nFailed due to a total of %d validation %s.\n", totalerrs, plural)
		os.Exit(1)
	}

	// Is a flag provided that we only should do configuration validation?
	if *flagValidateOnly {
		// if so, no point in continuing. Exit code 0 to indicate an a-okay.
		fmt.Printf("All configuration files (%d) are correctly validated:\n", len(*configurations))
		for _, c := range *configurations {
			fmt.Printf("  %s\n", c.FileName)
		}
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
func writeJSON(filename string, r *[]ConfigurationResult) error {
	b, err := json.MarshalIndent(r, "  ", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling json: %s", err)
	}

	err = ioutil.WriteFile(filename, b, 0644)
	if err != nil {
		return fmt.Errorf("unable to write to file `%s': %s\n", filename, err)
	}

	return nil
}

// Writes the slice of results to the given filename as CSV. If any error
// occurs, exit with code 1.
func writeCsv(filename string, results *[]ConfigurationResult) error {
	f, err := os.Create(filename)

	if err != nil {
		return fmt.Errorf("unable to open file for writing `%s': %s\n", filename, err)
	}

	w := csv.NewWriter(f)

	for _, r := range *results {
		for _, res := range r.Results {
			status := "FAIL"
			if res.Error == nil {
				status = "OK"
			}

			record := []string{
				status,
				res.Monitor.Name,
				res.Monitor.URL,
				strconv.FormatInt(res.Latency, 10),
			}
			w.Write(record)
		}
	}
	w.Flush()

	return nil
}

// Sanitizes Pandora Agent data. In Pandora, you can use certain macro's to fill a command after an alert.
// For instance, '_data_' is replaced with the module data. So if the module data contains the string:
//
//	assertion failed for regex `lala'
//
// then that text is replaced AS-IS for the _data_ string. Meaning if you want to execute a shell command,
// the command will also contain a backtick character, thus failing. This function sanitizes the data by
// removing the backtick (`), quote ('), double quote ("), backslash (\) character and exclamation point (!).
func sanitizePandoraData(s string) string {
    replacements := "`'\"\\!"
    for _, char := range replacements {
	s = strings.Replace(s, string(char), "", -1)
    }
    return s
}

// Writes all results to Pandora Agent interpretable XML files.
func writePandoraAgents(outdir string, results *[]ConfigurationResult) error {
	fmt.Printf("Writing %d configuration results to output directory '%s'\n", len(*results), outdir)

	for _, result := range *results {

		pfmsAgent := PfmsAgent{}
		pfmsAgent.AgentName = result.ConfigurationName
		pfmsAgent.GroupName = "Web Services" // ugh, currently hardcoded. Ah well, we'll fix that later.

		for _, actualResult := range result.Results {
			module := PfmsModule{}
			module.Name = actualResult.Monitor.Name
			module.Description = actualResult.Monitor.Description

			if actualResult.Error != nil {
				module.Data = sanitizePandoraData(actualResult.Error.Error())
				module.Type = "generic_data_string" // indicates string data
				module.Status = "CRITICAL"
			} else {
				module.Data = strconv.FormatInt(actualResult.Latency, 10)
				module.Type = "generic_data" // this indicates numeric data
				module.Status = "NORMAL"
			}

			pfmsAgent.Modules = append(pfmsAgent.Modules, module)
		}

		// write agent to file...
		xmlBytes, err := xml.MarshalIndent(pfmsAgent, " ", "   ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not marshal PFMS data to bytes: %s\n", err)
			os.Exit(1)
		}
		// As of PandoraFMS 5.0? The filename HAS to be named '$NAME.$TIMESTAMP.data' for some reason :/
		outputFile := fmt.Sprintf("%s.%d.data", result.ConfigurationName, time.Now().Unix())
		outputPath := path.Join(outdir, outputFile)
		err = ioutil.WriteFile(outputPath, xmlBytes, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not write to file: %s\n", err)
			os.Exit(1)
		}
	}

	return nil
}

// PfmsAgent is the root node when serializing PandoraFMS agent data.
type PfmsAgent struct {
	XMLName   struct{}     `xml:"agent_data"`
	AgentName string       `xml:"agent_name,attr"`
	GroupName string       `xml:"group,attr"`
	Modules   []PfmsModule `xml:"module"`
}

// PfmsModule contains information about a single module for PandoraFMS.
type PfmsModule struct {
	Name        string `xml:"name"`
	Type        string `xml:"type"`
	Description string `xml:"description,omitempty"`
	Data        string `xml:"data"`
	Status      string `xml:"status,omitempty"` // NORMAL, WARNING or CRITICAL
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

	for _, mon := range config.Monitor {
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
	ch := make(chan Result, len(config.Monitor))

	results := ConfigurationResult{}
	results.ConfigurationName = config.Name

	for _, mon := range config.Monitor {
		// fire all goroutines first
		if verbose {
			mon.Callback = verboseCallback
		}
		go mon.Run(filedir, ch)
	}

	// then receive from the channel
	for _ = range config.Monitor {
		result := <-ch
		results.Results = append(results.Results, result)
		fmt.Printf("%s\n", result)
	}

	return results
}

// Prints a short execution summary using all the results gathered.
func printExecutionSummary(configResults []ConfigurationResult) {
	var total int
	var countOk int
	var countFail int

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
		fmt.Fprintf(os.Stderr, "hmon version %s\n", VERSION)
		fmt.Fprintf(os.Stderr, `
A simplistic host monitor using content assertions. This tool connects to
configured http serving hosts, issues a request and checks the content using
regular expression 'assertions'. Requests can be sent with data, or without.
When data is sent, the HTTP method is automatically a POST. Without data,
the HTTP method will be a GET.

(Normal) output will always be written to the stdout. Using the flags -format
and -output, the tool can write to other output formats:

-format=json:    Javascript Object Notation
-format=csv:     Comma Separated Values
-format=pandora  PandoraFMS agent data (XML)

For more information, check the GitHub page at http://github.com/krpors/hmon.

FLAGS (with defaults):
`)
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
		writeFunc = writeJSON
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

	for _, c := range configurations {
		fmt.Printf("Processing configuration `%s' with %d monitors\n", c.Name, len(c.Monitor))

		// should we run in parallel?
		var cr ConfigurationResult
		if !*flagSequential {
			cr = runParallel(*flagFiledir, c, *flagVerbose)
		} else {
			// or sequential.
			cr = runSequential(*flagFiledir, c, *flagVerbose)
		}
		configResults = append(configResults, cr)

		fmt.Println()
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
