/*
A simplistic HTTP monitoring tool, using content assertions.

The idea is basically making a connection to one or multiple HTTP servers, issuing
a request (GET or POST), and run assertions on the response. The result of each
assertion is then returned, to check whether the response is in the expected format.

Configuration

The configuration itself is stored in an XML file, which should end in _hmon.xml.
Multiple configuration files can be created (must be in one directory). Hmon will
then read all _hmon.xml files, parse them, and validate them. Example configuration
file:

	<?xml version="1.0" encoding="UTF-8"?>
	<hmonconfig name="Example tests">
		<monitor name="Github.com">
			<url>https://status.github.com</url>
			<timeout>30000</timeout>
			<assertions>
				<assertion>GitHub System Status</assertion>
			</assertions>
		</monitor>
		<monitor name="Some web API check">
			<url>http://example.org/api</url>
			<file>someSoapRequest-1.0.xml</file>
			<timeout>30000</timeout>
			<headers>
				<header name="SOAPAction" value="http://soapaction"/>
			</headers>
			<assertions>
				<assertion>some kind of assertion string</assertion>
			</assertions>
		</monitor>
	</hmonconfig>

Each configuration file which is included in a run must have a unique 
hmonconfig/@name attribute. Furthermore, in a single configuration file,
the hmonconfig/monitor/@name attribute must also be unique. No worries though,
because the validate step should report any errors regarding uniqueness.

In each monitor node, you can specify the URL to send the request to using the
element <url>. If a <file> element is specified, the contents
of that specific file will be sent as HTTP POST data. Note that if the file is
NOT specified, a HTTP GET will be used instead. This may change in the future.
Using <timeout>, an optional timeout can be given, in milliseconds. If this 
node is not specified, the default value of 60 seconds is used. With <headers>
and its child elements <header>, custom HTTP headers can be sent. Think of Base64
authentication, or a SOAP action. Lastly, the <assertion> elements can be used
to specify regular expressions. The response is asserted against each of these
regexes. If one fails, hmon will report an error for that monitor.

Output

Generally, all output is reported to stdout. Additionally, other output
formats to file can be specified. Currently three different formats are 
supported: JSON, CSV and PandoraFMS agent data. PandoraFMS (see 
http://pandorafms.org) is a specialized output format in XML so the agent can
interprete it, and display it in the Pandora Web console.

Usable flags

The following flags can be used (defaults after the = sign):

	-combine=false

This will combine all single <monitor>s from all found configuration file.
In short, this mimics as if the x amount of configuration files are all
specified in one.

	-conf=""

Specify a SINGLE configuration file to run. If this flag is specified
(non empty), it will take precedence over -confdir.

	-confdir="."

The directory where the _hmon.xml files are stored. All files will be parsed
and validated, and are used to run all monitors within these files.

	-filedir="."

The base directory where all HTTP POST request data resides. The <file> 
node in the monitors will use this as base.

	-format=""

Output format. Three values can be given: 'json', 'csv', or 'pandora'.
The 'json' value will render the output to json, 'csv' will write the
results to comma separated values, and 'pandora' will write the results
to PandoraFMS agent specific XML data.

	-output=""

The output directory (in case of 'pandora' format) or output file (in case
of 'json' or 'csv').

	-sequential=false

When this flag is specified, all monitors from a configuration file are
executed sequentially, instead of spawning goroutines for each monitor. This
means every monitor waits for execution until the previous monitor is done.
Setting this flag is not recommended for monitor execution speed :)

	-validate=false
	
Validate configuration file(s) only.

	-verbose=false

Adds verbosity. Will print out request and responses for each monitor.

	-version=false

Prints out version information and exits.

Examples

A list of examples of running hmon:

	./hmon -confdir "./hmonconfigs/" -output "./results" -format pandora

Will search in ./hmonconfigs/ for _hmon.xml files. Output is written to ./results/
in the PandoraFMS format (files per configuration will be written).

	./hmon -confdir "./hmonconfigs/" -output "results.json" -format json

Will search in ./hmonconfigs/ for _hmon.xml files, and will write JSON output
to the results.json file.

	./hmon -confdir "./hmonconfigs/" -sequential

Will search in ./hmonconfigs/ for _hmon.xml files, and executes the monitors
sequentially, only writing output to stdout.
*/
package main
