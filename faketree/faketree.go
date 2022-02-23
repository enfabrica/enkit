package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"syscall"

	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/reexec"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/spf13/pflag"
)

type MountFlags struct {
	Source, Target string

	Flags  uintptr
	Fstype string
	Data   string
}

func (mf *MountFlags) Normalize() (*MountFlags, error) {
	target, err := RealPath(mf.Target)
	// Target may need to be created, ignore errors.
	if err != nil {
		target = mf.Target
	}

	source := mf.Source
	if source != "" {
		source, err = RealPath(mf.Source)
		if err != nil {
			return nil, fmt.Errorf("could not compute realpath of source %s: %w", mf.Source, err)
		}
	}

	retval := *mf
	retval.Target = target
	retval.Source = source
	return &retval, nil
}

func (mf *MountFlags) Mount() error {
	source := mf.Source
	if source == "" {
		source = mf.Fstype
		if source == "" {
			source = "none"
		}
	}
	return syscall.Mount(source, mf.Target, mf.Fstype, mf.Flags, mf.Data)
}

type MountOption struct {
	Name  string
	Value uintptr
}

type MountOptions []MountOption

func (mo MountOptions) Find(option string) *MountOption {
	for _, opt := range mo {
		if opt.Name == option {
			return &opt
		}
	}
	return nil
}

func (mo MountOptions) Serialize(flags uintptr, fstype, fsdata string) string {
	options := []string{}
	if flags != DefaultMountFlags {
		for _, opt := range mo {
			if (uintptr(opt.Value) & flags) > 0 {
				options = append(options, opt.Name)
			}
		}
	}
	if fstype != "" {
		options = append(options, "type="+fstype)
	}
	if fsdata != "" {
		options = append(options, "data="+fsdata)
	}

	return strings.Join(options, ",")
}

func (mo MountOptions) Parse(options string) (uintptr, string, string, error) {
	fields := strings.Split(options, ",")

	var fsflags uintptr
	var fstype, fsdata string

	var errs []error
	for ix, field := range fields {
		field = strings.TrimSpace(field)

		if t := strings.TrimPrefix(field, "type="); len(t) < len(field) {
			fstype = t
			continue
		}

		// data can only be specified last, anything (including ",") after data
		// is considered part of data.
		if d := strings.TrimPrefix(field, "data="); len(d) < len(field) {
			fsdata = strings.Join(append([]string{d}, fields[ix+1:]...), ",")
			break
		}

		option := KnownOptions.Find(field)
		if option == nil {
			errs = append(errs, fmt.Errorf("file system option #%d is unknown: %s", ix, field))
			continue
		}

		fsflags |= option.Value
	}

	return fsflags, fstype, fsdata, multierror.New(errs)
}

func (mo MountOptions) List() []string {
	list := []string{}
	for _, option := range mo {
		list = append(list, option.Name)
	}
	return list
}

var KnownOptions = MountOptions{
	{"dirsync", syscall.MS_DIRSYNC},
	{"mandlock", syscall.MS_MANDLOCK},
	{"noatime", syscall.MS_NOATIME},
	{"nodev", syscall.MS_NODEV},
	{"nodiratime", syscall.MS_NODIRATIME},
	{"noexec", syscall.MS_NOEXEC},
	{"nosuid", syscall.MS_NOSUID},
	{"ro", syscall.MS_RDONLY},
	{"recursive", syscall.MS_REC},
	{"relatime", syscall.MS_RELATIME},
	{"silent", syscall.MS_SILENT},
	{"strictatime", syscall.MS_STRICTATIME},
	{"sync", syscall.MS_SYNCHRONOUS},
	{"remount", syscall.MS_REMOUNT},
	{"bind", syscall.MS_BIND},
	{"shared", syscall.MS_SHARED},
	{"private", syscall.MS_PRIVATE},
	{"slave", syscall.MS_SLAVE},
	{"unbindable", syscall.MS_UNBINDABLE},
	{"move", syscall.MS_MOVE},
}

var DefaultMountFlags = uintptr(syscall.MS_BIND | syscall.MS_REC | syscall.MS_PRIVATE)

func NewMountFlags(mount string) (*MountFlags, error) {
	var source, target, data, fstype string

	flags := DefaultMountFlags
	splits := strings.SplitN(mount, ":", 3)
	switch len(splits) {
	default:
		return nil, fmt.Errorf("invalid mount: %s - format is '/source/path:/dest/path[:options]?'", mount)
	case 3:
		var err error
		flags, fstype, data, err = KnownOptions.Parse(splits[2])
		if err != nil {
			return nil, err
		}
		fallthrough
	case 2:
		target = splits[1]
		source = splits[0]
	}

	return &MountFlags{
		Source: source,
		Target: target,
		Flags:  flags,
		Fstype: fstype,
		Data:   data,
	}, nil
}

func (mf MountFlags) String() string {
	options := KnownOptions.Serialize(mf.Flags, mf.Fstype, mf.Data)
	if options != "" {
		options = ":" + options
	}

	return fmt.Sprintf("%s:%s%s", mf.Source, mf.Target, options)
}

func (mf *MountFlags) MakeTarget(perms os.FileMode) error {
	var err error
	var info os.FileInfo
	if mf.Source != "" {
		info, err = os.Stat(mf.Source)
	}

	var errs []error
	if err != nil || info == nil || info.IsDir() {
		if err := os.MkdirAll(mf.Target, perms); err != nil {
			errs = append(errs, fmt.Errorf("could not create target directory %s: %w", mf.Target, err))
		}
	} else if err == nil && !info.IsDir() {
		dirname := filepath.Dir(mf.Target)
		if err := os.MkdirAll(dirname, perms); err != nil {
			errs = append(errs, fmt.Errorf("could not create target directory for file mount %s: %w", dirname, err))
		} else {
			f, err := os.OpenFile(mf.Target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perms.Perm()&0o666)
			f.Close()

			if err != nil {
				errs = append(errs, fmt.Errorf("could not create target file mount %s: %w", mf.Target, err))
			}
		}
	}
	return multierror.New(errs)
}

type Flags struct {
	Fail     bool
	Root     bool
	Hostname string
	Chdir    string
	Faketree string
	Perms    uint32
	Proc     bool

	Uid, Gid int
	Mount    []MountFlags
}

// Args turns the content of the Flags object into a set of command line flags.
//
// Prefer this method over os.Args to generate the command line to spawn a new
// faketree instance to guarantee the use of normalized values.
//
// For example: using Args(), a uid supplied as a string username will be passed
// down as a numeric value, which is preferrable as within the newly spawned
// namespace there is no guarantee that the username will still resolve to the
// same uid.
func (opts *Flags) Args() []string {
	args := []string{"--uid", strconv.Itoa(opts.Uid), "--gid", strconv.Itoa(opts.Gid)}
	if opts.Root {
		args = append(args, "--root")
	}
	if opts.Fail {
		args = append(args, "--fail")
	}
	if opts.Hostname != "" {
		args = append(args, "--hostname", opts.Hostname)
	}
	if opts.Chdir != "" {
		args = append(args, "--chdir", opts.Chdir)
	}
	if opts.Faketree != "" {
		args = append(args, "--faketree", opts.Faketree)
	}
	if opts.Perms != kDefaultPerms {
		args = append(args, "--perms", fmt.Sprint(opts.Perms))
	}
	if opts.Proc {
		args = append(args, "--proc")
	}

	for _, mount := range opts.Mount {
		args = append(args, "--mount", mount.String())
	}
	return args
}

// ParseOrLookupUser returns an (uid, gid) for a string uid or username.
//
// For example: ParseOrLookupUser("daemon") will return (104, 104, nil)
// to indicate that it corresponds to uid 104, gid 104, with no error.
//
// If the uid is numeric, with for example ParseOrLookupUser("104"),
// group is returned as 0.
//
// An error is returned if the parameter is invalid, the user could
// not be looked up, or the look up returned invalid values.
func ParseOrLookupUser(uid string) (int, int, error) {
	i, err := strconv.Atoi(uid)
	if err == nil {
		if i >= 0 {
			return i, 0, nil
		}
		return 0, 0, fmt.Errorf("invalid uid: %d - must be >= 0", i)
	}

	u, err := user.Lookup(uid)
	if err != nil {
		return 0, 0, fmt.Errorf("could not lookup user: %s - %w", uid, err)
	}

	ud, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("lookup returned invalid uid: %s - %w", u.Uid, err)
	}

	gd, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("lookup returned invalid uid: %s - %w", u.Gid, err)
	}

	return ud, gd, nil
}

// ParseOrLookupGroup is like ParseOrLookupUser but for gids.
func ParseOrLookupGroup(gid string) (int, error) {
	i, err := strconv.Atoi(gid)
	if err == nil {
		if i >= 0 {
			return i, nil
		}
		return 0, fmt.Errorf("invalid gid: %d - must be >= 0", i)
	}

	group, err := user.LookupGroup(gid)
	if err != nil {
		return 0, fmt.Errorf("could not lookup group: %s - %w", gid, err)
	}
	gd, err := strconv.Atoi(group.Gid)
	if err != nil {
		return 0, fmt.Errorf("lookup returned invalid gid: %s - %w", gid, err)
	}
	return gd, nil
}

// RealPath returns the absolute path of a file/dir with all symlinks resolved.
func RealPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// Default permissions to use to create new directories or files.
const kDefaultPerms = 0o755

// Default exit code used to indicate an error in faketree itself.
const kDefaultExit = 125

func NewFlags() *Flags {
	flags := &Flags{
		Uid:   os.Getuid(),
		Gid:   os.Getgid(),
		Perms: kDefaultPerms,
	}

	// Realpath may fail due to how procfs is mounted.
	// In that case, there won't be a default for the faketree
	// path, and it'll be mandatory to specify one on the command line.
	path, _ := RealPath(reexec.Self())
	flags.Faketree = path
	return flags
}

// LogOrFail prints a log message or exits depends on fail.
func (opts *Flags) LogOrFail(msg string, args ...interface{}) {
	if opts.Fail {
		exit(fmt.Errorf(msg, args...))
	}
	log.Printf(msg, args...)
}

// Parses the specified command line arguments into a Flags object.
//
// Returns the arguments that were not parsed, or an error.
func (opts *Flags) Parse(argv []string) ([]string, error) {
	fs := pflag.NewFlagSet("faketree", pflag.ContinueOnError)

	fs.BoolVar(&opts.Root, "root", opts.Root, "Make the command believe it has root (will force uid=0 and gid=0 regardless of --uid and --gid options)")
	fs.BoolVar(&opts.Fail, "fail", opts.Fail, "Make fakeroot fail with an error in case any one of the setup steps fails. By default, faketree will continue.")
	fs.BoolVar(&opts.Proc, "proc", opts.Proc,
		"Don't ignore mounts of /proc, don't automatically mount /proc. "+
			"Faketree internally mounts /proc in order to work. "+
			"Given this, it will ignore any '--mount ...:/proc:...' request. "+
			"Use --proc if you instead want to mount /proc on your own with --mount, and "+
			"specify non standard options. Do so at your own risk, as faketree may no longer work.")

	fs.StringVar(&opts.Hostname, "hostname", opts.Hostname, "Make the command believe it is running on a different host name")
	fs.StringVar(&opts.Chdir, "chdir", opts.Chdir, "Change the current workingn directory to the one specified")
	fs.StringVar(&opts.Faketree, "faketree", opts.Faketree, "After partitions are mounted/readjusted, faketree needs to re-execute itself to drop privileges. "+
		"Given that the layout of the partitions has changed, it may be impossible for faketree to determine "+
		"its own path. If that's the case, you probably want to specify one manually using this option.")
	fs.Uint32Var(&opts.Perms, "perms", opts.Perms, "Permissions to use when creating directories. Use 0xxx or 0oxxx to indicate octal. "+
		"493 in decimal corresponds to 0o755")

	var uid, gid string
	fs.StringVar(&uid, "uid", strconv.Itoa(opts.Uid), "Make the command believe it is running as this uid")
	fs.StringVar(&gid, "gid", strconv.Itoa(opts.Gid), "Make the command believe it is running as this gid")

	var mounts []string
	fs.StringArrayVar(&mounts, "mount", nil, "Override the layout of the filesystem to have the specified directories mounted. "+
		"Syntax is: --mount path:destination:[options[,type=type]?[,data=...]?]?.")

	if err := fs.Parse(argv); err != nil {
		return nil, err
	}

	for _, mount := range mounts {
		m, err := NewMountFlags(mount)
		if err != nil {
			return nil, err
		}
		opts.Mount = append(opts.Mount, *m)
	}

	var err error
	if !opts.Root {
		if uid != "" {
			opts.Uid, opts.Gid, err = ParseOrLookupUser(uid)
			if err != nil {
				return nil, err
			}
		}

		if gid != "" {
			opts.Gid, err = ParseOrLookupGroup(gid)
			if err != nil {
				return nil, err
			}
		}
	} else {
		opts.Uid, opts.Gid = 0, 0
	}

	return fs.Args(), nil
}

func initializeSystem() {
	flags := NewFlags()
	left, err := flags.Parse(os.Args[1:])
	if err != nil {
		exit(err)
	}

	if flags.Hostname != "" {
		if err := syscall.Sethostname([]byte(flags.Hostname)); err != nil {
			flags.LogOrFail("Error setting hostname - %s\n", err)
		} else {
			os.Setenv("HOSTNAME", flags.Hostname)
		}
	}

	for _, omount := range flags.Mount {
		mount, err := omount.Normalize()
		if err != nil {
			flags.LogOrFail("Skipping mount %s - %v", omount, err)
			continue
		}
		if !flags.Proc && (mount.Target == "/proc" || mount.Target == "/proc/") {
			flags.LogOrFail("Skipping mount %s - proc is automatically mounted (unless --proc is used)", omount)
			continue
		}

		mkerr := mount.MakeTarget(os.FileMode(flags.Perms))
		if err := mount.Mount(); err != nil {
			if mkerr != nil {
				flags.LogOrFail("Could not create mount target %s - %v", mount.Target, mkerr)
			}
			flags.LogOrFail("Could not mount %s - %v", mount, err)
		}
	}

	// Why is this necessary? Mostly to unconfuse golang libraries.
	//
	// When the UidMappings and GidMappings are used, the /proc/$pid/uid_map and
	// /proc/$pid/gid_map files must be updated. The golang exec library does this
	// internally and transparently, but...
	//
	// When PID namespaces are used, the child process has a different view of PID
	// numbers compared to the parent. Eg, getpid() in the child will
	// return an integer completely different from what the parent has, possibly
	// assigned to a different process in a different namespace.
	//
	// If /proc is not re-mounted in the child namespace, it will have /proc/$pid/...
	// directories based on whoever mounted it last? so accessing /proc/$child_pid/...
	// will fail, or point to the wrong process.
	//
	// This is generally a non-issue as processes tend to access their own data info
	// through /proc/self/... which works regardless.
	//
	// But UidMappings and GidMappings are changing parameters for a 3rd party process,
	// so /proc/... MUST have the correct PID directories for the specific namespace.
	if !flags.Proc {
		mount := MountFlags{
			Target: "/proc",
			Fstype: "proc",
			// Default flags on an ubunut/debian box.
			Flags: syscall.MS_RELATIME | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID,
		}
		if err := mount.Mount(); err != nil {
			flags.LogOrFail("Could not mount %s - %v", mount, err)
		}
	}

	enterPrivileges(flags, left)
}

func initializePrivileges() {
	flags := NewFlags()
	left, err := flags.Parse(os.Args[1:])
	if err != nil {
		exit(err)
	}

	if err := syscall.Setuid(flags.Uid); err != nil {
		flags.LogOrFail("Error changing to uid %d - %s\n", flags.Uid, err)
	}

	if err := syscall.Setgid(flags.Gid); err != nil {
		flags.LogOrFail("Error changing to gid %d - %s\n", flags.Gid, err)
	}

	if flags.Chdir != "" {
		merr := os.MkdirAll(flags.Chdir, os.FileMode(flags.Perms))
		if err := os.Chdir(flags.Chdir); err != nil {
			exit(fmt.Errorf("Could not chdir to %s - as specified with --chdir - error was: %w. "+
				"Attempting to create the directory resulted in %w", flags.Chdir, err, merr))
		}
		os.Setenv("PWD", flags.Chdir)
	}

	Exec(left...)
}

// DefaultShell returns the default shell as per environment variables, or "/bin/sh".
func DefaultShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "/bin/sh"
	}
	return shell
}

// Exec calls exec() with the specified arguments.
func Exec(args ...string) {
	if len(args) == 0 {
		args = []string{DefaultShell(), "--norc", "--noprofile"}
	}

	binary, err := exec.LookPath(args[0])
	if err != nil {
		exit(fmt.Errorf("Error finding the %s command - %w", args[0], err))
	}

	env := append(os.Environ(), "FAKETREE=true")
	if err := syscall.Exec(binary, args, env); err != nil {
		exit(fmt.Errorf("Error running the binary %s - %v command - %s", binary, args, err))
	}
}

// NextCommand creates an exec.Cmd to run the next command in the pipeline.
func NextCommand(name string, flags *Flags, left []string) *exec.Cmd {
	args := []string{name}
	args = append(args, flags.Args()...)
	args = append(args, "--")
	args = append(args, left...)

	cmd := reexec.Command(args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func enterSystem() {
	flags := NewFlags()
	left, err := flags.Parse(os.Args[1:])
	if err != nil {
		exit(err)
	}

	cmd := NextCommand("initialize-system", flags, left)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | // Isolate PIDs. Necessary for a /proc mount to work.
			syscall.CLONE_NEWNS | // independent set of mounts.
			syscall.CLONE_NEWUTS | // host and domain names.
			syscall.CLONE_NEWIPC | // sysv ipc
			syscall.CLONE_NEWUSER, // new user namespace

		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	exit(cmd.Run())
}

var kHelpScreen = `
faketree spawns a command so it runs with its own independent view of the
file system, but with the same uid and privileges as the user who originally
started the command.

For example:

     faketree --mount /var/log:/tmp/log --chdir /tmp/log -- /bin/sh
         Will return a shell in a directory hierarchy as the one of the
	 system where faketree was started, but with /tmp/log mapped to
	 the original /var/log. When run as user marx, the shell will show:

	   $ id
	   uid=1000(marx) gid=1000(marx)
	   $ pwd
	   /tmp/log
	   $ realpath /tmp/log
	   /tmp/log
	   $ ls /tmp/log
	   ... same as ls /var/log

     faketree --mount /var/log:/tmp/log --chdir /tmp/log -- ls
         Runs the command 'ls' instead of /bin/sh.

     faketree --mount /opt/data/build-0014:/opt/build \
              --mount /opt/data/build-0014/logs:/var/log \
              --mount /opt/data/build-0014/bin:/usr/bin \
              --mount /opt/data/build-0014/sbin:/usr/sbin \
	      --chdir /opt/build -- sh -c "make; make install"
         Runs the commands make and make install in a file system view
	 that has /usr/bin, /usr/sbin, /var/log, ... mapped into the
	 corresponding directories in /opt/build.

Mount syntax:

  The --mount option defaults to performing the equivalent of
  'mount --rbind source-path destination-path'.

  Additional options can be specified with
     '--mount source:dest:[option[,type=...]?[,data=...]?]*'

  With this syntax:
    - If any option is specified, all options must be specified.
      Eg, if you need to bind a directory in read only mode, you must
      specify: '--mount source:dest:recursive,bind,ro'.
    - Leave source "empty" to mount file systems that don't have
      a source file/device. For example, to mount a tmpfs file system,
      use '--mount :/destination/dir:type=tmpfs'.
    - data=..., if specified, MUST be last. It allows to pass arbitrary
      string options down to the file system layer.
    - Internally, faketree needs /proc/ to be mounted and will mount it
      automatically. Any request to mount /proc/ will be ignored, unless
      --proc is specified, in which case a '--mount :proc:/proc:...' flag
      must be supplied, otherwise faketree will fail to start.
    - Most mount(8) options are supported, with the similar semantics:
      ` + strings.Join(KnownOptions.List(), ",") + `
`

func exit(err error) {
	if err == nil {
		os.Exit(0)
	}

	var eerr *exec.ExitError
	if errors.As(err, &eerr) {
		os.Exit(eerr.ExitCode())
	}

	if errors.Is(err, pflag.ErrHelp) {
		fmt.Fprintf(os.Stderr, kHelpScreen)
		os.Exit(kDefaultExit)
	}

	log.Printf("FAILED: %v", err)
	os.Exit(kDefaultExit)
}

func enterPrivileges(flags *Flags, left []string) {
	cmd := NextCommand("initialize-privileges", flags, left)

	cmd.Path = flags.Faketree
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER, // new user namespace

		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: flags.Uid,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: flags.Gid,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	exit(cmd.Run())
}

func main() {
	// Namespaces require the use of clone() to create a new child process
	// into a new, isolated, namespace. clone() is a fork equivalent, which is
	// unsafe to call in multithreaded programs unless immediately followed
	// by exec().
	//
	// The Golang APIs support namespaces through SysProcAttr in cmd.Exec,
	// which enforces the requirement above by immediately executing an external
	// program.
	//
	// To continue the set up of the environment, which requires multiple
	// steps, the common workaround is to re-execute the same binary.
	//
	// To move the program forward, the code below builds a state machine
	// where the state is represented by argv[0], and uses the docker
	// reexec library to associate a function to a state name.
	//
	// At time of writing:
	// - argv[0]=unrecognized -> enterSystem.
	//      NextCommand("initialize-system")
	// - argv[0]=initialize-system -> initializeSystem, enterPrivileges.
	//      NextCommand("initialize-privileges")
	// - argv[0]=initialize-privileges -> initializePrivilieges
	//      Exec(... command or shell ...)
	reexec.Register("initialize-system", initializeSystem)
	reexec.Register("initialize-privileges", initializePrivileges)
	if !reexec.Init() {
		enterSystem()
	}
}
