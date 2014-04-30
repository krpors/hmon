# hmon - a simplistic http monitor using content assertions

The idea is basically:

1. Connect to site/service over http or https
1. Use GET or POST with data (think SOAP, yuck)
1. Check content with configured expected response string/regex.
1. Give feedback to user.
1. Optionally write out to different needed formats.

# Why?

At work, I'm maintaining several of these ugly SOAP services. Right now I'm periodically
checking the state of these services using SoapUI, which is IMHO a heavy-weight piece of
crap. I wanted something configurable using the command-line and Go. The production servers
already have some kind of monitoring, but our (4) non-production servers do not.

# Documentation

Please refer to http://godoc.org/github.com/krpors/hmon for in depth documentation (which
is generated via the `doc.go` file).
