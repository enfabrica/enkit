package main

import (
	"context"
	"os"
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
		SilenceUsage:  true,
		SilenceErrors: true,

		Args:  cobra.NoArgs,

		Long: `"staco post" adds or updates a stable comment on github.

To use 'staco post' you must supply --github-owner, --github-repo and
--pr to select the github repository and PR to work on.

For authentication purposes, you must provide a --github-token (or
a GH_TOKEN environment variable). If you use the gh tool from
https://github.com/cli/cli you can configure the token with:

  export GH_TOKEN=$(gh auth status -t 2>&1 |sed -ne 's/.*Token: //p')

To describe the stable comment to add or update, you should specify
the options --template, --json, and one of the --diff-* options.

To understand how to use those options, you need to understand
how post works:
1) It checks if the PR specified already has a stable comment
   that was posted before (see --marker below). If it does, it loads
   the --template and --json that was used on the last update.
   If you are guaranteed that a comment already exists,
   --template and --json are thus entirely optional.

2) If --template was specified on the command line, or no template
   was found in the posted comment, the template on the command
   line is used. --template OVERRIDES the one on the PR - so you
   can always roll out new dashboards easily.

3) If --json was specified on the command line, and no json was
   found in the comment, than json is used. --json provides a
   default json if no other comment was pushed before.

4) Finally, --diff-{patch,merge,jd} describe how to change the
   comment, in the format chosen.
   One of the --diffs options is generally always required,
   unless you are pushing a static comment - one you don't
   plan to dynamically update.

Stateless scripts updating a dashboard concurrently from a
CI/CD pipeline will typically always provide a --template and 
an empty/skeleton --json, and always update the content via
--diff. See the examples section.`,
		Example: `
  $ staco post --json '{}' --template '{{. |printf "%#v"}}' \
       --github-owner octo-test --github-repo octo-repo --pr 3 \
       --dry-run

    Creates (or updates) a stable comment for PR#3 on the
    repository https://github.com/octo-test/octo-repo.
    The comment posted will just dump the content of the json
    as computed by staco. No change is applied.
    --dry-run prevents any change, so no PR is harmed by
    running this command.

  $ staco post --json '{"Runs":[]}' --template '{{. |printf "%#v"}}' \
       --diff-patch '[{"op":"add", "path":"/Runs/0", "value":{"Test": 123}}]' \

    Same as above, except no PR and no github repository is specified,
    so the command is assumed to be running in dry-run mode, and outputs
    what it would do on the screen. Also, it modifies the json to prepend
    {"Test":123} to the "Runs" array.

    The first time this command is run, "Runs" will be initialized to an
    empty array, and immediately gain the {"Test":123} first value. If this
    command is re-run multiple times (on an actual PR), the "Runs"
    array will keep growing with more and more {"Test":123} objects.`,
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
		diff, err := github.NewDiffFromFlags(df)
		if err != nil {
			return err
		}

		sc, err := github.NewStableComment(github.StableCommentFromFlags(scf))
		if err != nil {
			return err
		}

		var repo *github.RepoClient
		if pr != 0 {
			repo, err = github.NewRepoClient(github.RepoClientFromFlags(context.Background(), gh))
			if err != nil {
				return err
			}

			if err := sc.UpdateFromPR(repo, pr); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: no PR specified, --dryrun is assumed - just showing result on STDOUT\n")
			dryrun = true
		}

		if !dryrun && repo != nil {
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

		SilenceUsage:  true,
		SilenceErrors: true,

		Example: `
  $ staco diff --input='{"Runs":[]}' --output='{"Runs":[{"Test":123}]}'

        Outputs the diff betwee input and output in all supported formats.
        Specifically, in the example above, it would output:

  diff in jd format:
  @ ["Runs",-1]
  + {"Test":123}
  
  diff in merge format:
  {"Runs":[{"Test":123}]}
  diff in patch format:
  [{"op":"add","path":"/Runs/-","value":{"Test":123}}]`,
	}

	var input, output string
	cmd.Flags().StringVar(&input, "input", "", "Input JSON")
	cmd.Flags().StringVar(&output, "output", "", "Output JSON")

	var jdfmt, patchfmt, mergefmt bool
	cmd.Flags().BoolVar(&jdfmt, "jd", false, "Shows the diff in jd format (and no other, unless explicitly enabled)")
	cmd.Flags().BoolVar(&patchfmt, "patch", false, "Shows the diff in patch format (and no other, unless explicitly enabled)")
	cmd.Flags().BoolVar(&mergefmt, "merge", false, "Shows the diff in merge format (and no other, unless explicitly enabled)")

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

		SilenceUsage:  true,
		SilenceErrors: true,

		Long: `"staco show" parses and shows the staco metadata in a PR.

The command will scan all the comments of a PR, look for the staco
comment with the specified --marker, and output the metdata
stored in the comment itself: the json, and template used.

This command is mostly useful for debugging, or in cases
the built in patching support is not enough. For example,
you could use "staco show ... --json=true | jd ... > new.json"
to edit the json with more familiar commands, to then
pipe it back to the post command.`,
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

		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(PostCommand())
	root.AddCommand(ShowCommand())
	root.AddCommand(DiffCommand())

	cobra.EnablePrefixMatching = true
	kcobra.Run(root)
}
