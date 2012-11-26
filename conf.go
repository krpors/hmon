package main

// Root configuration node, contains zero or more Monitor
// structs.
type Config struct {
	Monitor []Monitor `xml:"monitor"`
}

// Monitor node with its children. All slices can be zero or more,
// technically, but logically some kind of validation should be done :/
type Monitor struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"desc,attr"`
	Url         string   `xml:"url"`
	File        string   `xml:"req"`
	Timeout     int      `xml:"timeout"`
	Headers     []Header `xml:"headers>header"`
	Assertions  []string `xml:"assertions>assertion"`
}

// Extra HTTP headers to send.
type Header struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}
