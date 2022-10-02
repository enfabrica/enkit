package main

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/github"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/josephburnett/jd/lib"
	"github.com/spf13/cobra"
)

func PostCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "post --github-... --pr=PR# --template='...' --diff-...",
		Short: "Posts data to a new or existing stable comment",
		Args:  cobra.NoArgs,
	}

	gh := github.DefaultRepoClientFlags()
	gh.Register(&kcobra.FlagSet{cmd.Flags()}, "")

	df := &github.StableCommentDiffFlags{}
	df.Register(&kcobra.FlagSet{cmd.Flags()}, "")

	scf := github.DefaultStableCommentFlags()
	scf.Register(&kcobra.FlagSet{cmd.Flags()}, "")

	var pr int
	cmd.Flags().IntVar(&pr, "pr", 0, "PR Number to update")
	var dryrun bool
	cmd.Flags().BoolVarP(&dryrun, "dry-run", "n", false, "Don't change the comment, show what you would do")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if pr == 0 {
			return kflags.NewUsageErrorf("A PR number MUST be specified with --pr")
		}
		repo, err := github.NewRepoClient(github.RepoClientFromFlags(context.Background(), gh))
		if err != nil {
			return err
		}

		diff, err := github.NewDiffFromFlags(df)
		if err != nil {
			return err
		}

		sc, err := github.NewStableComment(github.StableCommentFromFlags(scf))
		if err != nil {
			return err
		}

		if err := sc.UpdateFromPR(repo, pr); err != nil {
			return err
		}
		if !dryrun {
			if err := sc.PostToPR(repo, diff, pr); err != nil {
				return err
			}
		} else {
			result, err := sc.PreparePayloadFromDiff(diff)
			if err != nil {
				return err
			}
			fmt.Printf("On PR %d - would %s - content:\n===========\n%s\n==========\n", pr, sc.PostAction(), result)
		}
		return nil
	}

	return cmd
}

func DiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff --input '{...}' --output '{...}'",
		Short: "Shows the diff between the input and output json in patch/merge/jd format",
		Args:  cobra.NoArgs,
	}

	var input, output string
	cmd.Flags().StringVar(&input, "input", "", "Input JSON")
	cmd.Flags().StringVar(&output, "output", "", "Output JSON")

	var jdfmt, patchfmt, mergefmt bool
	cmd.Flags().BoolVar(&jdfmt, "jd", false, "Shows the diff in jd format (and no other, unless explicitly enabled)")
	cmd.Flags().BoolVar(&patchfmt, "patch", false, "Shows the diff in patch format (and no other, unless explicitly enabled)")
	cmd.Flags().BoolVar(&mergefmt, "merge", false, "Shows the diff in merge format (and no other, unless explicitly enabled)")

	// FIXME: http://play.jd-tool.io/
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if input == "" {
			return kflags.NewUsageErrorf("You MUST specify an input JSON string")
		}
		if output == "" {
			return kflags.NewUsageErrorf("You MUST specify an output JSON string")
		}

		inj, err := jd.ReadJsonString(input)
		if err != nil {
			return fmt.Errorf("Invalid --input json: %w", err)
		}
		outj, err := jd.ReadJsonString(output)
		if err != nil {
			return fmt.Errorf("Invalid --output json: %w", err)
		}

		if !jdfmt && !patchfmt && !mergefmt {
			jdfmt, patchfmt, mergefmt = true, true, true
		}

		diff := inj.Diff(outj)
		if jdfmt {
			out := diff.Render()
			if patchfmt || mergefmt {
				fmt.Printf("diff in jd format:\n")
			}
			fmt.Printf("%s\n", out)
		}
		if mergefmt {
			diff := inj.Diff(outj, jd.MERGE)

			out, err := diff.RenderMerge()
			if err != nil {
				out = fmt.Sprintf("(cannot be rendered in merge format - %s)", err)
			}
			if jdfmt || mergefmt {
				fmt.Printf("diff in merge format:\n")
			}
			fmt.Printf("%s\n", out)
		}
		if patchfmt {
			out, err := diff.RenderPatch()
			if err != nil {
				out = fmt.Sprintf("(cannot be rendered in patch format - %s)", err)
			}
			if jdfmt || mergefmt {
				fmt.Printf("diff in patch format:\n")
			}
			fmt.Printf("%s\n", out)
		}

		return nil
	}
	return cmd
}

func ShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Downloads and shows the data of a posted stable comment",
		Args:  cobra.NoArgs,
	}

	gh := github.DefaultRepoClientFlags()
	gh.Register(&kcobra.FlagSet{cmd.Flags()}, "")

	var pr int
	cmd.Flags().IntVar(&pr, "pr", 0, "PR Number to update")
	var marker string
	cmd.Flags().StringVar(&marker, "marker", github.DefaultMarker, "A unique marker to identify the comment across subsequent runs of this command")

	var showjson, showtemplate, showid bool
	cmd.Flags().BoolVar(&showjson, "json", false, "Enable to show the json (and nothing else, unless explicitly enabled)")
	cmd.Flags().BoolVar(&showtemplate, "template", false, "Enable to show the template (and nothing else, unless explicitly enabled)")
	cmd.Flags().BoolVar(&showid, "id", false, "Enable to show the id (and nothing else, unless explicitly enabled)")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if pr == 0 {
			return kflags.NewUsageErrorf("A PR number MUST be specified with --pr")
		}
		repo, err := github.NewRepoClient(github.RepoClientFromFlags(context.Background(), gh))
		if err != nil {
			return err
		}

		sc, err := github.NewStableComment(github.WithMarker(marker))
		if err != nil {
			return err
		}

		id, payload, template, err := sc.FetchPRState(repo, pr)
		if err != nil {
			return err
		}

		if !showjson && !showtemplate && !showid {
			showjson, showtemplate, showid = true, true, true
		}
		if id == 0 {
			return fmt.Errorf("no stable comment found for this PR")
		}

		if showid {
			if showjson || showtemplate {
				fmt.Printf("comment id: ")
			}
			fmt.Printf("%d\n", id)
		}
		if showtemplate {
			if showjson || showid {
				fmt.Printf("template:\n")
			}
			fmt.Printf("%s\n", template)
		}
		if showjson {
			if showtemplate || showid {
				fmt.Printf("json:\n")
			}
			fmt.Printf("%s\n", payload)
		}
		return nil
	}

	return cmd
}

func main() {
	root := &cobra.Command{
		Use:   "staco",
		Short: "Tool to create and handle stable comments on github",
	}

	root.AddCommand(PostCommand())
	root.AddCommand(ShowCommand())
	root.AddCommand(DiffCommand())

	cobra.EnablePrefixMatching = true
	kcobra.Run(root)
}
