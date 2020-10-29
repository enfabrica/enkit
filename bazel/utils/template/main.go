package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"text/template"
)

type ArrayFlag []string

func (i *ArrayFlag) String() string {
	return fmt.Sprintf("%q", *i)
}

func (i *ArrayFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type ArrayFileFlag []string

func (i *ArrayFileFlag) String() string {
	return fmt.Sprintf("%q", *i)
}

func (i *ArrayFileFlag) Set(value string) error {
	data, err := ioutil.ReadFile(value)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", value, err)
	}
	*i = append(*i, string(data))
	return nil
}

type MapWithGet map[string]string

func (m MapWithGet) Get(key string) (string, error) {
	v, found := m[key]
	if !found {
		return "", fmt.Errorf("key %s - could not be found", key)
	}
	return v, nil
}

func main() {
	templatefile := flag.String("template", "", "Path of a template file to expand")
	outputfile := flag.String("output", "", "Path of output file to generate")
	executable := flag.Bool("executable", false, "Set to true if you want the output file to be marked as executable")

	keys := []string{}
	flag.Var((*ArrayFlag)(&keys), "key", "Key to be expanded")
	values := []string{}
	flag.Var((*ArrayFlag)(&values), "value", "value to be expanded")
	flag.Var((*ArrayFileFlag)(&values), "valuefile", "value to be expanded")

	flag.Parse()

	if *templatefile == "" {
		log.Fatal("Must provide a template file with -template")
	}

	if len(keys) != len(values) {
		log.Fatalf("The number of keys passed with -key must equal the number of values with -value (%d != %d)", len(keys), len(values))
	}

	var err error
	outfd := os.Stdout
	if *outputfile != "" {
		perm := os.FileMode(0664)
		if *executable {
			perm = os.FileMode(0775)
		}
		outfd, err = os.OpenFile(*outputfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			log.Fatalf("File %s cannot be created: %s", *outputfile, err)
		}
		defer outfd.Close()
	}

	templatedata, err := ioutil.ReadFile(*templatefile)
	if err != nil {
		log.Fatalf("Template file %s cannot be read - %s", *templatefile, err)
	}

	t, err := template.New(*templatefile).Parse(string(templatedata))
	if err != nil {
		log.Fatalf("Cannot parse template %s - %s", templatedata, err)
	}

	subs := map[string]string{}
	for ix := range keys {
		subs[keys[ix]] = values[ix]
	}

	if err := t.Execute(outfd, MapWithGet(subs)); err != nil {
		log.Printf("Expansions:")
		for k, v := range subs {
			log.Printf("  %s=%s", k, v)
		}
		log.Fatalf("Could not expand template %s - %s", templatedata, err)
	}
}
