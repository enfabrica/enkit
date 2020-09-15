package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/client/astore"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"strings"
)

type TagCommand struct {
	*cobra.Command
	root *Root

	name string
	op   func([]string) astore.TagModifier
}

func NewTagCommand(root *Root, name string, op func([]string) astore.TagModifier) *TagCommand {
	command := &TagCommand{
		Command: &cobra.Command{
			Use:   fmt.Sprintf("%s UID tag [tag]...", name),
			Short: fmt.Sprintf("%ss the specified tags", strings.Title(name)),
		},
		root: root,
		name: name,
		op:   op,
	}
	command.Command.RunE = command.Run
	return command
}

func (uc *TagCommand) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return kflags.NewUsageErrorf("use as 'astore tag %s UID tag [tag]...' - the UID of exactly one artifact, followed by one or more tags", uc.name)
	}

	uid := args[0]
	tags := args[1:]

	client, err := uc.root.StoreClient()
	if err != nil {
		return err
	}

	arts, err := client.Tag(uid, uc.op(tags))
	if err != nil {
		return err
	}

	uc.root.OutputArtifacts(arts)
	return nil
}

type Tag struct {
	*cobra.Command
}

func NewTag(root *Root) *Tag {
	command := &Tag{
		Command: &cobra.Command{
			Use:   "tag",
			Short: "Mingles with the tags assigned to artifacts",
		},
	}

	command.Command.AddCommand(NewTagCommand(root, "add", astore.TagAdd).Command)
	command.Command.AddCommand(NewTagCommand(root, "del", astore.TagDel).Command)
	command.Command.AddCommand(NewTagCommand(root, "set", astore.TagSet).Command)

	return command
}
