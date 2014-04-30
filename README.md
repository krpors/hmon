# hmon - a simplistic http monitor using content assertions

The idea is basically:

1. Connect to site/service over http or https
1. Use GET or POST with data (think SOAP, yuck)
1. Check content with configured expected response string/regex.
1. Give feedback to user.
1. Optionally write out to different needed formats.

# Why?

At work, I'm maintaining several SOAP services. Checking the uptime of these
services by manually invoking with, let's say SoapUI, works beautifully. I wanted
something on the commandline however, which produces results to JSON, CSV or other
format which can be displayed periodically.

# Documentation

Please refer to  GoDoc:

[![GoDoc](https://godoc.org/github.com/krpors/hmon?status.png)](https://godoc.org/github.com/krpors/hmon)
