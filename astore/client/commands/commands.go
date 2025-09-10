package commands

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/enfabrica/enkit/astore/client/astore"
	castore "github.com/enfabrica/enkit/astore/client/astore"
	arpc "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/config"
	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	formatterMap = map[string]castore.Formatter{
		"pretty-json": NewStructuredStdout(&marshal.PrettyJsonEncoder{}),
		"json": NewStructuredStdout(&marshal.JsonEncoder{}),
		"toml": NewStructuredStdout(&marshal.TomlEncoder{}),
		"yaml": NewStructuredStdout(&marshal.YamlEncoder{}),
		"gob":  NewStructuredStdout(&marshal.GobEncoder{}),
	}
)

type Root struct {
	*cobra.Command
	*client.BaseFlags

	store         *client.ServerFlags
	outputFile    string
	consoleFormat string
}

func New(base *client.BaseFlags) *Root {
	root := NewRoot(base)

	root.AddCommand(NewDownload(root).Command)
	root.AddCommand(NewUpload(root).Command)
	root.AddCommand(NewList(root).Command)
	root.AddCommand(NewGuess(root).Command)
	root.AddCommand(NewTag(root).Command)
	root.AddCommand(NewNote(root).Command)
	root.AddCommand(NewPublic(root).Command)
	return root
}

func NewRoot(base *client.BaseFlags) *Root {
	rc := &Root{
		Command: &cobra.Command{
			Use:           "astore",
			Short:         "Push, pull, and publish build artifacts",
			SilenceUsage:  true,
			SilenceErrors: true,
			Example: `  $ astore upload build.out
        To upload a file in the artifact repository.

  $ astore upload build.out@experiments/builds/
        Same as above, stores the file in experiments/build.

  $ astore download experiments/builds/build.out
        Downloads the latest version of build.out.

  $ astore --help
        To have a nice help screen.`,
			Long: `astore - uploads and downloads artifacts`,
		},
		BaseFlags: base,
		store:     client.DefaultServerFlags("store", "Artifacts store metadata server", ""),
	}

	rc.store.Register(&kcobra.FlagSet{FlagSet: rc.PersistentFlags()}, "")
	rc.Command.PersistentFlags().StringVarP(&rc.outputFile, "meta-file", "m", "",
		fmt.Sprintf("Meta-data output file. Supported formats: %s", marshal.Formats()))
	rc.Command.PersistentFlags().StringVar(
		&rc.consoleFormat,
		"console-format",
		"table",
		fmt.Sprintf("Format to use for stdout output. Supported formats: %s", append([]string{"table"}, marshal.Formats()...)),
	)

	return rc
}

func (rc *Root) StoreClient() (*astore.Client, error) {
	if rc.outputFile != "" {
		// check output file type is supported
		marshaller := marshal.ByExtension(rc.outputFile)
		if marshaller == nil {
			return nil, fmt.Errorf("Output file extension not supported `%s`.  Supported formats: %s",
				rc.outputFile, marshal.Formats())
		}

		// check that the destination is writable
		file, err := os.Create(rc.outputFile)
		if err != nil {
			return nil, fmt.Errorf("Problems creating output file `%s` - %w", rc.outputFile, err)
		}
		file.Close()
	}

	_, cookie, err := rc.IdentityCookie()
	if err != nil {
		return nil, err
	}

	storeconn, err := rc.store.Connect(client.WithCookie(cookie))
	if err != nil {
		return nil, err
	}

	return astore.New(storeconn), nil
}

func (rc *Root) Formatter(mods ...Modifier) astore.Formatter {
	// The table formatter doesn't follow the same interface as the others, and
	// can't be constructed until this point. This code should only be called once
	// per command, making modification of this global OK; even so, it overwrites
	// the "table" value each time so should behave as expected.
	formatterMap["table"] = NewTableFormatter(mods...)

	formatterList := NewFormatterList()

	if format, ok := formatterMap[strings.ToLower(rc.consoleFormat)]; ok {
		formatterList.Append(format)
	} else {
		// Fall back to the table formatter
		formatterList.Append(formatterMap["table"])
	}

	if rc.outputFile != "" {
		// add a marshal-aware formatter
		formatterList.Append(NewOpFile(rc.outputFile))
	}

	return formatterList
}

func (rc *Root) OutputArtifacts(arts []*arpc.Artifact) {
	formatter := rc.Formatter(WithNoNesting)
	for _, art := range arts {
		formatter.Artifact(art)
	}
	formatter.Flush()
}

func (rc *Root) ConfigStore(namespace ...string) (config.Store, error) {
	return defcon.Open("astore", namespace...)
}

type Download struct {
	*cobra.Command
	root *Root

	ForceUid  bool
	ForcePath bool
	Output    string
	Overwrite bool
	Arch      string
	Tag       []string
}

func SystemArch() string {
	return strings.ToLower(runtime.GOARCH + "-" + runtime.GOOS)
}

func NewDownload(root *Root) *Download {
	command := &Download{
		Command: &cobra.Command{
			Use:     "download <path|uid>...",
			Short:   "Downloads an artifact",
			Aliases: []string{"down", "get", "pull", "fetch"},
		},
		root: root,
	}
	command.Command.RunE = command.Run

	command.Flags().BoolVarP(&command.ForceUid, "force-uid", "u", false, "The argument specified identifies an uid")
	command.Flags().BoolVarP(&command.ForcePath, "force-path", "p", false, "The argument specified identifies a file path")
	command.Flags().StringVarP(&command.Output, "output", "o", ".", "Where to output the downloaded files. If multiple files are supplied, a directory with this name will be created")
	command.Flags().BoolVarP(&command.Overwrite, "overwrite", "w", false, "Overwrite files that already exist")
	command.Flags().StringArrayVarP(&command.Tag, "tag", "t", []string{"latest"}, "Download artifacts matching the tag specified. More than one tag can be specified")
	command.Flags().StringVarP(&command.Arch, "arch", "a", SystemArch(), "Architecture to download the file for")

	return command
}
func (dc *Download) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return kflags.NewUsageErrorf("use as 'astore download <path|uid>...' - one or more paths to download")
	}
	if dc.ForceUid && dc.ForcePath {
		return kflags.NewUsageErrorf("cannot specify --force-uid together with --force-path - an argument can be either one, but not both")
	}

	mode := astore.IdAuto
	if dc.ForceUid {
		mode = astore.IdUid
	}
	if dc.ForcePath {
		mode = astore.IdPath
	}

	var archs []string
	switch strings.TrimSpace(dc.Arch) {
	case "":
		fallthrough
	case "all":
		archs = []string{"all"}
	default:
		archs = []string{dc.Arch, "all"}
	}

	// If there are multiple files to download, the output must be a directory.
	// Append a trailing '/' so one will be created if necessary.
	output := dc.Output
	if len(args) > 1 && output != "" {
		output = output + "/"
	}

	ftd := []astore.FileToDownload{}
	for _, name := range args {
		file := astore.FileToDownload{
			Remote:       name,
			RemoteType:   mode,
			Local:        output,
			Overwrite:    dc.Overwrite,
			Architecture: archs,
			Tag:          &dc.Tag,
		}
		ftd = append(ftd, file)
	}

	dc.root.Log.Debugf("Files to download: %+v", ftd)

	client, err := dc.root.StoreClient()
	if err != nil {
		return err
	}

	arts, err := client.Download(ftd, astore.DownloadOptions{
		Context: dc.root.BaseFlags.Context(),
	})
	if err != nil && os.IsExist(err) {
		return fmt.Errorf("file already exists? To overwrite, pass the -w or --overwrite flag - %s", err)
	}

	formatter := dc.root.Formatter()
	for _, art := range arts {
		formatter.Artifact(art)
	}
	return err
}

type List struct {
	*cobra.Command
	root *Root

	Tag []string
	All bool
}

func NewList(root *Root) *List {
	command := &List{
		Command: &cobra.Command{
			Use:     "list",
			Short:   "Shows artifacts",
			Aliases: []string{"list", "show", "ls", "find"},
		},
		root: root,
	}
	command.Command.RunE = command.Run
	command.Flags().StringArrayVarP(&command.Tag, "tag", "t", []string{"latest"}, "Restrict the output to artifacts having this tag")
	command.Flags().BoolVarP(&command.All, "all", "l", false, "Show all binaries")

	return command
}

func (l *List) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return kflags.NewUsageErrorf("use as 'astore list [PATH]' - with a single, optional, PATH argument (got %d arguments)", len(args))
	}
	query := ""
	if len(args) == 1 {
		query = args[0]
	}

	client, err := l.root.StoreClient()
	if err != nil {
		return err
	}

	tags := l.Tag
	if l.All {
		tags = []string{}
	}
	options := astore.ListOptions{
		Context: l.root.BaseFlags.Context(),
		Tag:     tags,
	}

	arts, els, err := client.List(query, options)
	if err != nil {
		return err
	}

	formatter := l.root.Formatter()
	for _, art := range arts {
		formatter.Artifact(art)
	}
	if !l.All && len(arts) >= 1 {
		l.root.Log.Warnf("(only showing artifacts with %d tags: %v - use --all or -l to show all)\n", len(l.Tag), l.Tag)
	}

	for _, el := range els {
		formatter.Element(el)
	}
	formatter.Flush()

	return nil
}

type SuggestFlags astore.SuggestOptions

func (sf *SuggestFlags) Register(flagset *pflag.FlagSet) {
	flagset.StringVarP(&sf.Directory, "directory", "d", "", "Remote directory where to upload each file. If not specified explicitly, a path will be guessed using other heuristics")
	flagset.StringVarP(&sf.File, "file", "f", "", "Remote file name where to store all files. This is useful when uploading multiple files of different architectures")
	flagset.BoolVarP(&sf.DisableGit, "disable-git", "G", false, "Don't use the git repository to name the remote file")
	flagset.BoolVarP(&sf.DisableAt, "disable-at", "A", false, "Don't use the @ convention to name the remote file")
	flagset.BoolVarP(&sf.AllowAbsolute, "allow-absolute", "b", false, "Allow absolute local paths to name remote paths")
	flagset.BoolVarP(&sf.AllowSingleElement, "allow-single", "l", false, "Allow a single element path to be used as remote")
}

func (sf *SuggestFlags) Options() *astore.SuggestOptions {
	return (*astore.SuggestOptions)(sf)
}
