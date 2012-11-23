# hmon - a simplistic host monitor using content assertions

The idea is basically:
1. Connect to site/service over http or https
1. Use GET or POST with data (think SOAP, yuck)
1. Check content with configured expected response string/regex.
1. Give feedback to user.

# Why?

At work, I'm maintaining several of these ugly SOAP services. Right now I'm periodically
checking the state of these services using SoapUI, which is IMHO a heavy-weight piece of
crap. I wanted something configurable using the command-line and Go. The production servers
already have some kind of monitoring, but our (4) non-production servers do not.

# Example configuration

Although I generally dislike XML, I still chose to do the configuration in it. Perhaps
later it'll be something else, who knows. YAML?

This'll be a working draft, but this is the main idea:

    <?xml version="1.0" encoding="UTF-8"?>
    <hmon>
        <http>
            <url>http://www.iana.org/domains/example/</url>
            <!-- optional, default is 30 secs? -->
            <timeout>60</timeout>
            <!-- optional extra headers to send -->
            <headers>
                <header name="SOAPAction" value="whatevs"/>
            </headers>
            <assertions>
                <!-- regexes -->
                <assert>Example Domains</assert>
            </assertions>
        </http>
    </hmon>

There should be the possibility to do configuration in several files, instead of one 
big thing. The program will be given a directory to search for configurations, and parse
them all.

Validation must be done during parsing so users (a.k.a. me) knows if there's a fault.
