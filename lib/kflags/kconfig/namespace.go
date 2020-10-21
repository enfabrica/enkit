package kconfig

import (
	"bytes"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"text/template"
)

// Key is the name of a flag, Retriever is an object capable of retrieving its value.
type paramIndex map[string][]Retriever

type implementation struct {
	*Implementation
	path string
}

type namespaceData struct {
	params   paramIndex
	commands []Command
	hidden   bool

	imp implementation
}

type namespaceIndex map[string]*namespaceData

func (nsi namespaceIndex) Get(name string) *namespaceData {
	data := nsi[name]
	if data != nil {
		return data
	}
	data = &namespaceData{}
	nsi[name] = data
	return data
}

type NamespaceAugmenter struct {
	index namespaceIndex
	cf    CommandFactory

	log     logger.Logger
	mangler kflags.EnvMangler

	wg    sync.WaitGroup
	elock sync.RWMutex // Protects errs below, but also access to visited flags (which may not support concurrent access).
	errs  []error      // Collects errors generated asynchronously by the downloader. Use only under lock.
}

// ParamFactory is a function capable of creating a Retriever (or returning an error...) for a given config Parameter.
type ParamFactory func(*Parameter) (Retriever, error)

// CommandFactory is a function capable of retrieving a command implementation.
type CommandFactory func(url, hash string) (string, *Manifest, error)

func NewNamespaceAugmenter(namespaces []Namespace, log logger.Logger, mangler kflags.EnvMangler, cf CommandFactory, pf ParamFactory) (*NamespaceAugmenter, error) {
	ci := &NamespaceAugmenter{index: map[string]*namespaceData{}, cf: cf, log: log, mangler: mangler}
	errs := []error{}
	for _, ns := range namespaces {
		_, found := ci.index[ns.Name]
		if found {
			errs = append(errs, fmt.Errorf("command %s - defined multiple times in config - will only consider the first definition", ns.Name))
			continue
		}

		pi := paramIndex{}
		for dx, def := range ns.Default {
			params, _ := pi[def.Name]

			retriever, err := pf(&ns.Default[dx])
			if err != nil {
				errs = append(errs, err)
				continue
			}

			params = append(params, retriever)
			pi[def.Name] = params
		}
		*ci.index.Get(ns.Name) = namespaceData{
			params:   pi,
			hidden:   ns.Hidden,
			commands: ns.Command,
		}
	}

	return ci, multierror.New(errs)
}

// Example:
//
// VisitCommand("enkit")
// enkit --help
// - --help should show the availability of a docker command, complete with description.
// - How? Config expanding the "enkit" namespace by adding the "docker" command.
//
// VisitCommand("enkit.docker")
// enkit docker --help
//  - --help should show the availability of a "dev" and "fpga" containers, complete with description.
//    Further, the syntax should allow for specifying a specific version / tag.
//
//  - How? Config expanding the "enkit.docker" namespace, adds the ... dev and fpga commands.
//    Ideally, without downloading them. Just based on the config file, creates the command, sets
//    an implementation, configures some flags.
//
// VisitCommand("enkit.docker.dev")
// enkit docker dev --help
//  - --help should show the availability of run, login and upgrade. It should show the -t flag
//    to download a specific tagged version.
//
//  - How? dev has an implementation. It is downloaded and waited for. The inner manifest file is
//    parsed, defining extra commands and flags.
//
// VisitCommand("enkit.docker.dev")
// enkit docker dev -t latest --help
//  - --help should show the availability of run, login, and upgrade, with their flags.
//
//  - How? dev has an implementation.
// enkit docker dev login
// enkit docker dev login
// enkit docker dev upgrade
//
// enkit setup
// enkit enroll
//
// Implementation:
// - enkit: nil implementation
//   - AddCommand("enkit.docker", definition, flags, nil) -> if implementation is nil, adds a placeholder.
//
// - enkit.docker:
//   - AddCommand("enkit.docker.dev", definition, flags, nil) -> if implementation is nil, adds a placeholder.
//   - AddCommand("enkit.docker.fpga", definition, flags, nil) -> if implementation is nil, adds a placeholder.
//
// - enkit.docker.dev:
//   - DOWNLOAD PACKAGE
//   - AddCommand("enkit.docker.dev.run", definition, flags, runCommand) ->
//   - AddCommand("enkit.docker.dev.login", definition, flags, loginCommand) ->
//   - AddCommand("enkit.docker.dev.upgrade", definition, flags, upgradeCommand) ->

func BuildMap(vars []Var, flagarg []kflags.FlagArg) map[string]interface{} {
	subs := map[string]interface{}{}
	for _, v := range vars {
		subs[v.Key] = v.Value
	}
	for _, v := range flagarg {
		subs[v.Name] = v.Value.String()
	}

	wd, err := os.Getwd()
	if err == nil {
		subs["invoked_dir"] = wd
	}

	epath, err := os.Executable()
	if err == nil {
		subs["binary_dir"] = epath
	}

	return subs
}

func ExpandArgs(argv []string, subs map[string]interface{}) ([]string, error) {
	res := []string{}
	for _, arg := range argv {
		buffer := &bytes.Buffer{}
		err := template.Must(template.New("'"+arg+"'").Option("missingkey=error").Parse(arg)).Execute(buffer, subs)
		if err != nil {
			return nil, err
		}

		res = append(res, buffer.String())
	}
	return res, nil
}

func PrepareEnv(subs map[string]interface{}, mangler kflags.EnvMangler) []string {
	env := os.Environ()
	for k, v := range subs {
		str, ok := v.(string)
		if !ok {
			continue
		}

		k = mangler(k)
		if k == "" {
			continue
		}

		env = append(env, k+"="+str)
	}
	return env
}

func CreateExecuteAction(packagedir string, commanddir string, argv []string, vars []Var, mangler kflags.EnvMangler, printer logger.Printer) (kflags.CommandAction, error) {
	if len(argv) < 1 {
		return nil, fmt.Errorf("argv must have at least the command name - it is empty")
	}

	return func(flagarg []kflags.FlagArg, args []string) error {
		subs := BuildMap(vars, flagarg)
		subs["package_dir"] = packagedir
		subs["command_dir"] = commanddir

		argv, err := ExpandArgs(argv, subs)
		if err != nil {
			return err
		}

		program := filepath.Join(commanddir, argv[0])

		command := exec.Command(program, argv[1:]...)
		command.Env = PrepareEnv(subs, mangler)
		command.Dir = packagedir

		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr

		printer("running %s", command.String())
		if err := command.Run(); err != nil {
			printer("command %s failed with %s", err)
		}
		return err
	}, nil
}

func (c *NamespaceAugmenter) VisitCommand(namespace string, command kflags.Command) (bool, error) {
	nsIndex, found := c.index[namespace]
	if !found {
		return false, nil
	}
	command.Hide(nsIndex.hidden)

	commander, ok := command.(kflags.Commander)
	if !ok {
		return true, nil
	}

	type icommand struct {
		path string
		Command
	}

	commands := []icommand{}
	prepare := func(dir string, toadd []Command) {
		for _, c := range toadd {
			commands = append(commands, icommand{Command: c, path: dir})
		}
	}

	prepare(nsIndex.imp.path, nsIndex.commands)
	if impl := nsIndex.imp.Implementation; impl != nil && impl.Package != nil {
		dir, manifest, err := c.cf(impl.Package.URL, impl.Package.Hash)
		if err != nil {
			return false, err
		}
		if manifest != nil {
			prepare(dir, manifest.Command)
		}
	}

	var errs []error
	for _, extra := range commands {
		var action kflags.CommandAction
		if impl := extra.Implementation; impl != nil {
			if len(impl.Local) > 0 && len(impl.System) > 0 {
				errs = append(errs, fmt.Errorf("command %s is invalid: defines both a local and system action - %#v", extra.CommandDefinition.Name, *impl))
				continue
			}

			if impl.Package != nil {
				intns := namespace + "." + extra.Name
				c.index.Get(intns).imp = implementation{
					Implementation: extra.Implementation,
					path:           extra.path,
				}
			}

			var err error
			if len(impl.Local) > 0 {
				action, err = CreateExecuteAction(extra.path, extra.path, impl.Local, impl.Var, c.mangler, c.log.Infof)
			} else if len(impl.System) > 0 {
				action, err = CreateExecuteAction(extra.path, "", impl.System, impl.Var, c.mangler, c.log.Infof)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}

		if err := commander.AddCommand(extra.CommandDefinition, extra.Flag, action); err != nil {
			errs = append(errs, err)
		}
	}
	return true, multierror.New(errs)
}

func (c *NamespaceAugmenter) VisitFlag(namespace string, flag kflags.Flag) (bool, error) {
	nsIndex, found := c.index[namespace]
	if !found {
		return false, nil
	}

	param, found := nsIndex.params[flag.Name()]
	if !found {
		return false, nil
	}

	setter := func(origin, value string, err error) {
		c.elock.Lock()
		defer c.elock.Unlock()

		c.wg.Done()
		if err != nil {
			c.errs = append(c.errs, err)
			return
		}
		if err := flag.SetContent(origin, []byte(value)); err != nil {
			c.errs = append(c.errs, fmt.Errorf("could not set flag '%s', value '%s' caused %w", flag.Name(), value, err))
		}
	}

	for _, p := range param {
		c.wg.Add(1)
		p.Retrieve(setter)
	}

	return true, nil
}

func (c *NamespaceAugmenter) Done() error {
	c.wg.Wait()

	// Not necessary - all downloads have completed by now. Here in case tsan is not smart enough.
	defer c.elock.RUnlock()
	c.elock.RLock()
	return multierror.New(c.errs)
}
