package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/client/astore"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PublicAdd struct {
	root *Root
	*cobra.Command

	Uid           string
	NonExistentOK bool
	Arch          string
	Tag           []string
	All           bool
}

func NewPublicAdd(root *Root) *PublicAdd {
	command := &PublicAdd{
		root: root,
		Command: &cobra.Command{
			Use:   "add",
			Short: "Makes the specified artifact available at the URL provided",
		},
	}
	command.Command.RunE = command.Run

	command.Flags().BoolVarP(&command.NonExistentOK, "non-existent-ok", "x", false, "It's ok if this artifact does not exist right now. As soon as the artifact appears, the URL will work")
	command.Flags().StringVarP(&command.Arch, "arch", "a", "", "Architecture to return at this URL. If empty, the client will be able to select the architecture")
	command.Flags().StringVarP(&command.Uid, "uid", "u", "", "Limit to only this specific uid, regardless of other parameters")

	command.Flags().StringArrayVarP(&command.Tag, "tag", "t", []string{"latest"}, "Restrict the output to artifacts having this tag")
	command.Flags().BoolVarP(&command.All, "all", "l", false, "Show all binaries")

	return command
}

func (uc *PublicAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 && len(args) != 2 {
		return kflags.NewUsageErrorf("use as 'astore public add <uid|path> [path]' - one or two arguments")
	}

	artifact := args[0]
	destination := artifact
	if len(args) > 1 {
		destination = args[1]
	}

	tags := uc.Tag
	if uc.All {
		tags = []string{}
	}

	toPublish := astore.ToPublish{
		Public:        destination,
		Path:          artifact,
		Architecture:  uc.Arch,
		NonExistentOK: uc.NonExistentOK,
		Tag:           &tags,
	}

	client, err := uc.root.StoreClient()
	if err != nil {
		return err
	}

	url, meta, err := client.Publish(toPublish)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return fmt.Errorf("A published path by the name of '%s' already exists - use unpublish to remove it", destination)
		}
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("No path '%s' could be found on server - nothing to publish. Use --non-existent-ok to publish a pending url", toPublish.Path)
		}
		return err
	}

	formatter := uc.root.Formatter(WithHeading(fmt.Sprintf("Published at %s", url)))
	for _, art := range meta.Artifact {
		formatter.Artifact(art)
	}
	formatter.Flush()
	return err
}

type PublicDel struct {
	root *Root
	*cobra.Command
}

func NewPublicDel(root *Root) *PublicDel {
	command := &PublicDel{
		root: root,

		Command: &cobra.Command{
			Use:     "del",
			Short:   "Makes the specified public URL unavailable",
			Aliases: []string{"rm"},
		},
	}
	command.Command.RunE = command.Run
	return command
}

func (uc *PublicDel) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return kflags.NewUsageErrorf("use as 'astore public del <url>...' - one or more urls to unpublish")
	}

	client, err := uc.root.StoreClient()
	if err != nil {
		return err
	}

	for _, arg := range args {
		if err := client.Unpublish(arg); err != nil {
			return err
		}
	}

	return nil
}

type Public struct {
	*cobra.Command
}

func NewPublic(root *Root) *Public {
	command := &Public{
		Command: &cobra.Command{
			Use:     "public",
			Short:   "Adds/Remove a public URL to access an artifact",
			Aliases: []string{"publish", "share"},
		},
	}

	command.Command.AddCommand(NewPublicAdd(root).Command)
	command.Command.AddCommand(NewPublicDel(root).Command)

	return command
}
