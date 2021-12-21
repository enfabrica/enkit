package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func Copy(dst, src string) error {
	fsrc, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("could not open source file - %w", err)
	}
	defer fsrc.Close()

	fdst, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return fmt.Errorf("could not open destination file - %w", err)
	}
	defer fdst.Close()

	_, err = io.Copy(fdst, fsrc)
	if err != nil {
		return err
	}

	if err := fdst.Sync(); err != nil {
		return err
	}

	return fdst.Close()
}

var (
	moduleRe = regexp.MustCompile(`^module\s+(.*?)\n`)
)

func main() {
	path := flag.String("path", "", "Top level directory where all the repositories are stored (eg, appengine/test/test-deploy-dir)")
	entry := flag.String("entry", "", "Directory project containing your app (eg, github.com/ccontavalli/myapp)")
	config := flag.String("config", "", "Path to the app.yaml file to use for the deploy")
	gomod := flag.String("gomod", "", "Path to a go.mod file to prepare for deploy")
	gosum := flag.String("gosum", "", "Path to a go.sum file to prepare for deploy")
	gcloud := flag.String("gcloud", "/usr/bin/gcloud", "Path to the gcloud binary to use")
	extra := flag.String("extra", "", "Extra flags to pass to gcloud")
	quiet := flag.Bool("quiet", false, "Be more quiet")
	flag.Parse()

	if *entry == "" || *config == "" {
		flag.Usage()
		os.Exit(1)
	}

	project := filepath.Join(*path, "src", *entry)
	if s, err := os.Stat(project); err != nil || !s.Mode().IsDir() {
		log.Fatalf("Couldn't enter directory supplied with --entry=%s: %s directory caused %s", *entry, project, err)
	}

	if *gomod != "" {
		b, err := ioutil.ReadFile(*gomod)
		if err != nil {
			log.Fatalf("failed to read %q: %v", *gomod, err)
		}
		m := moduleRe.FindSubmatch(b)
		if len(m) < 2 {
			log.Fatalf("failed to find module in go.mod")
		}
		projectRoot := filepath.Join(*path, "src", strings.TrimSpace(string(m[1])))
		fmt.Fprintf(os.Stderr, "Copying go.mod to %q\n", projectRoot)
		dest := filepath.Join(projectRoot, "go.mod")
		if err := Copy(dest, *gomod); err != nil {
			log.Fatalf("Couldn't copy %s - supplied with -gomod - to %s: %s", *gomod, projectRoot, err)
		}
		if *gosum != "" {
			fmt.Fprintf(os.Stderr, "Copying go.sum to %q\n", projectRoot)
			dest := filepath.Join(projectRoot, "go.sum")
			if err := Copy(dest, *gosum); err != nil {
				log.Fatalf("Couldn't copy %s - supplied with -gosum - to %s: %s", *gosum, projectRoot, err)
			}
		}
	}

	if *config != "" {
		dest := filepath.Join(project, "app.yaml")
		if err := Copy(dest, *config); err != nil {
			log.Fatalf("Couldn't copy %s - supplied with -config - to %s: %s", *config, dest, err)
		}
	}

	fmt.Fprintf(os.Stderr, "Deploying %q to cloud\n", project)

	if !*quiet {
		cmd := exec.Command("/usr/bin/find", ".")
		cmd.Dir = *path
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("find failed: %s", err)
		}
	}

	if *gcloud != "" {
		*extra = strings.TrimSpace(*extra)
		var cmd *exec.Cmd
		if *extra != "" {
			cmd = exec.Command("/bin/sh", "-c", fmt.Sprintf("%s app deploy %s", *gcloud, *extra))
		} else {
			cmd = exec.Command(*gcloud, "app", "deploy")
		}
		cmd.Dir = filepath.Join(project)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("%s failed: %s", *gcloud, err)
		}
	}
}