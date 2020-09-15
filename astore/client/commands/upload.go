package commands

import (
	"github.com/enfabrica/enkit/astore/client/astore"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
)

type Upload struct {
	*cobra.Command
	root *Root

	Suggest SuggestFlags
	Arch    string
	Note    string
	Tag     []string
}

func NewUpload(root *Root) *Upload {
	command := &Upload{
		Command: &cobra.Command{
			Use:   "upload [localFile[@remoteName]]...",
			Short: `Uploads one or more artifacts`,
			Long: `Uploads one or more artifacts.

This command uploads the specified list of files in your cloud
artifact repository.

To use this command, you must first run 'astore login' to authenticate.

When uploading a file, the tool needs to know:
1) The path on your machine where the file to upload is, the LOCAL path.
2) How you want to name this file in your cloud repository, the REMOTE name.
3) If this file is only supposed to be used on some architectures, its architecture.

The LOCAL path can be an absolute path, or a relative path. As long as the
tool can open the path and read its content, you are good to go.

The REMOTE path can only be a relative path. It specifies where you want
this file to be stored. If an artifact by the chosen name already exists,
uploading will replace its latest version, but both the new version and
old version will still be available.

a) You can specify it explicitly, using the @ notation. For example, to
   upload the file /etc/hosts and name it dns/configrations/hosts, you
   can run 'astore upload /etc/hosts@dns/configurations/hosts', eg,
   'astore upload LOCAL@REMOTE'.

   If the REMOTE destination specified ends with '/', it is assumed you
   want to treat it as a directory. The name of the file uploaded will
   be preserved, but stored in the specified REMOTE directory.
   For example, 'astore upload /etc/hosts@dns/configurations/' is equivalent
   to '... /etc/hosts@dns/configurations/hosts'.

b) You can specify a destination path with -d. For example,
   'astore upload -d dns/configurations /etc/hosts' will result in
   the file being stored as dns/configurations/hosts.

c) Let astore figure it out. It has a bunch of heuristics built in.
   If you are in a git repository, for example, a relative path
   will be turned into 'repository-name/path/to/the/file'.

d) Use a relative path. Relative paths are preserved as is in the
   remote repository.

For the architecture:

a) You can use the -a option, and specify an architecture explicitly.
   For example, with '-a i386-linux' or '-a amd64-mac'.

b) Let astore figure it out. It has a bunch of heuristics to read
   PE, ELF, and MACHO files to guess the architecture.

c) If no architecture is guessed or specified, it is assumed that the
   file can run on any architecture, 'all' is used.`,
			Example: `  $ astore upload ./test/file.bin
	Will upload the file './test/file.bin' and store it as 'test/file.bin'.
  $ astore upload /etc/hosts@global/configs/hosts
	Store the file '/etc/hosts' as 'global/configs/hosts'. If you use an absolute
        path, you must specify the remote name.
  $ astore upload /etc/hosts@global/configs/
	Same as above, configs/ ends with a /, it is assumed to be a directory.
  $ astore upload /etc/hosts@global/configs
	Store the file '/etc/hosts' as 'global/configs'. This is perfectly legal,
	although undesired. A REMOTE path can be an artifact, and contain a set
	of directories at the same time.
  $ astore upload -d configs /etc/hosts
	Store the file '/etc/hosts' in the configs directory, as 'configs/hosts'.
  $ astore upload -f tools/play play-amd64-linux play-amd64-darwin play-amd64-windows
	Store the files play-amd64-linux, play-amd64-darwin and play-amd64-windows all as
        the file 'tools/play' in the repository, with different arch tags.

  $ astore upload -n "This is only a test, do not use in production" /etc/hosts@configs/
	Similar to previous commands, but annotate the binary with a note
	that will be displayed at every list and download.
  $ astore upload -t kernel:2.6.0 -t debug-binary /etc/hosts@configs/
	Similar to previous commands, but assign tags to the binary, available
	for querying.
`,
			Aliases: []string{"up", "put", "push", "send"},
		},
		root: root,
	}
	command.Command.RunE = command.Run

	command.Suggest.Register(command.Flags())
	command.Flags().StringVarP(&command.Arch, "arch", "a", "", "Architecture of the file, avoid automated detection")
	command.Flags().StringVarP(&command.Note, "note", "n", "", "Note to add to the upload")
	command.Flags().StringArrayVarP(&command.Tag, "tag", "t", nil, "Tags to assign to the binary being uploaded")

	return command
}

func (uc *Upload) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return kflags.NewUsageErrorf("use as 'astore upload <file>...' - one or more paths to upload")
	}

	client, err := uc.root.StoreClient()
	if err != nil {
		return err
	}

	options := astore.UploadOptions{
		Context: uc.root.BaseFlags.Context(),
	}

	files := []astore.FileToUpload{}
	for _, arg := range args {
		local, remote, err := astore.SuggestRemote(arg, *uc.Suggest.Options())
		if err != nil {
			return err
		}

		architectures := []string{uc.Arch}
		if uc.Arch == "" {
			arch, err := astore.GuessArchOS(local)
			if err != nil {
				architectures = []string{"all"}
			} else {
				architectures = astore.ToArchArray(arch)
			}
		}

		files = append(files, astore.FileToUpload{Local: local, Remote: remote, Architecture: architectures, Note: uc.Note, Tag: uc.Tag})
	}
	arts, err := client.Upload(files, options)
	if err != nil {
		return err
	}

	uc.root.OutputArtifacts(arts)
	return nil
}
