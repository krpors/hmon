# hmon - a simplistic host monitor using content assertions

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

# Usage

Per `./hmon -help`:

hmon version 0.1

A simplistic host monitor using content assertions. This tool connects to
configured http serving hosts, issues a request and checks the content using
regular expression 'assertions'.

Normal output is done to the standard output, and using the flag -outfile
combined with -outtype the results can be written to different file formats.

For more information, check the GitHub page at http://github.com/krpors/hmon.

FLAGS:
-confdir=".": Directory with configurations of \*\_hmon.xml files.
-filedir=".": Base directory to search for request files. If ommited, the current working directory is used.
-format="": Output format ('csv', 'html', 'json'). Only suitable in combination with -outfile .
-outfile="": Output to given file. If empty, output will be done to stdout only.
-sequential=false: When set, execute monitors in sequential order (not recommended for speed).
-validate=false: When specified, only validate the configuration file(s), but don't run the monitors.
-version=false: Prints out version number and exits (discards other flags).

# Example configuration

Although I generally dislike XML, I still chose to do the configuration in it. Perhaps
later it'll be something else, who knows.

The current draft has the following format:

    <?xml version="1.0" encoding="UTF-8"?>
    <hmonconfig name="Tries a few websites">
        <monitor name="Github.com">
            <url>https://status.github.com</url>
            <!-- <file>content.xml</file> -->
            <timeout>60</timeout>
            <headers>
                <header name="Name" value="Cruft"/>
            </headers>
            <assertions>
                <assertion>GitHub System Status</assertion>
            </assertions>
        </monitor>
    </hmonconfig>

Some explanation:

A configuration file should end in `_hmon.xml`.

Unlimited amount of `monitor` elements can be nested inside the `hmonconfig` root.

* `hmonconfig@name` is required, and specifies the name of the configuration that's run.
* `hmonconfig/monitor@name` is also required, for output purposes.
* `hmonconfig/monitor/url` is the URL to send the request to.
* `hmonconfig/monitor/file` is optional. When specified, a HTTP POST will be done with the
file as contents. If ommitted, a HTTP GET is done.
* `hmonconfig/monitor/timeout` is the timeout for the request. Currently not implemented,
but will be mapped in the code.
* `hmonconfig/monitor/headers` specify the additional HTTP headers to send. Optional.
* `hmonconfig/monitor/assertions/` specify the assertions to check the response with. These
should be full-fledged regular expressions.

The configuration above issues a request to https://status.github.com, with no HTTP POST data.
One HTTP header is added: `Name`=`Cruft`. This will result in a HTTP GET, and the response
is asserted to one regular expression: `GitHub System Status`. 

Given that this configuration is saved in `config_hmon.xml`:

    $ ls
    hmon  config_hmon.xml
    $ ./hmon -confdir="."
    Processing configuration `Tries a few websites' with 1 monitors
    ok    Github.com (515.4369ms)

    Executed 1 monitors with 1 successes and 0 failures
