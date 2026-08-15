package main

import (
	"flag"
	"fmt"
)

const harnessVersion = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("harness " + harnessVersion)
		return
	}

	fmt.Println("not implemented yet")
}
