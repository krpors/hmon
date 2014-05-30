/*
S to H: SoapUI to Hmon converter.

Very simple SoapUI project file to hmon configuration converter. This tool
currently only works with SoapUI projects with WSDLs, testsuites and testcases.
Normal HTTP calls are not (yet) supported.  To invoke the tool, supply SoapUI
project file as the first argument to the tool. It will generate two folders:
	
	configs

This folder contains the generated hmon configuration file. The filename is
based on the name of the testcase.

	postdata

This folder contains one subdirectory named after the testsuite. That folder
contains XML files which are the postdata.

*/
package main
