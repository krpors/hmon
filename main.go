package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
)

func main() {
	contents, err := ioutil.ReadFile("conf.xml")
	if err != nil {
		fmt.Errorf("Fail!")
	}

	c := Config{}
	err = xml.Unmarshal(contents, &c)
	if err != nil {
		fmt.Errorf(err.Error())
	}

	for _, cfg := range c.Monitor {
		fmt.Println(cfg.File)
		fmt.Println(cfg.Url)
		fmt.Println(cfg.Timeout)
		fmt.Println(cfg.Headers[0].Name, cfg.Headers[0].Value)
		fmt.Println(cfg.Assertions[0])
	}
}
