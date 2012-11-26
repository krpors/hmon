package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	contents, err := ioutil.ReadFile("conf.xml")
	if err != nil {
		fmt.Errorf("Fail!")
		os.Exit(1)
	}

	c := Config{}
	err = xml.Unmarshal(contents, &c)
	if err != nil {
		fmt.Errorf(err.Error())
	}

	err = c.Validate()
	if err != nil {
		verr := err.(ValidationError)
		fmt.Println(verr)
		for _, e := range verr.ErrorList {
			fmt.Println("  ", e)
		}
	}
}
