package main

import (
	"github.com/enfabrica/enkit/lib/config/marshal"

	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

func main() {
	format := flag.String("format", "", "Format of the file to validate")
	output := flag.String("output", "", "If set to a path, the validated path will be written here")
	strip := flag.String("strip", "", "If set, an extension to strip to guess the format")
	flag.Parse()

	args := flag.Args()
	if len(args) <= 0 {
		log.Fatal("Must supply list of files to validate")
	}

	var marshaller marshal.FileMarshaller
	if *format != "" {
		marshaller = marshal.ByFormat(*format)
		if marshaller == nil {
			log.Fatalf("Format specified with --format %s is unknown - %v", *format, marshal.Formats())
		}
	}

	var err error
	var outputfd *os.File
	if *output != "" {
		outputfd, err = os.OpenFile(*output, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0664)
		if err != nil {
			log.Fatalf("Could not open output file %s - %s", *output, err)
		}
		defer outputfd.Close()
	}
	for _, input := range args {
		marshaller := marshaller
		if marshaller == nil {
			guess := input
			if *strip != "" {
				guess = strings.TrimSuffix(guess, *strip)
			}
			marshaller = marshal.ByExtension(guess)
			if marshaller == nil {
				log.Fatalf("File %s has unknown format - known formats are %v", input, marshal.Formats())
			}
		}
		file, err := ioutil.ReadFile(input)
		if err != nil {
			log.Fatalf("Could not read file %s: %s", input, err)
		}
		var data interface{}
		if err := marshaller.Unmarshal(file, &data); err != nil {
			log.Fatalf("File %s contains errors: %s", input, err)
		}

		if outputfd != nil {
			if _, err := outputfd.Write(file); err != nil {
				log.Fatalf("Failed writing in output file %s: %s", *output, err)
			}
		}
	}
}
