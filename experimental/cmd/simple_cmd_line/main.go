package main

import (
	"flag"
	"fmt"
)

var (
	intFlag    = flag.Int("int_flag", 0, "Example int flag")
	stringFlag = flag.String("string_flag", "foo", "Example string flag")
)

func main() {
	flag.Parse()

	fmt.Println("simple command line")
	fmt.Printf("Int flag value: %d\n", *intFlag)
	fmt.Printf("String flag value: %q\n", *stringFlag)
}
