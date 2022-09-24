package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/josephburnett/jd/lib"
	"regexp"
	"text/template"
)

// StableComment is an object to manipulate and process stable github comments.
//
// A "stable" github comment is a comment attached to a PR that is periodically
// updated to show some important information about the PR.
//
// For example, it can be used by a BOT to compute a list of reviewers, and
// as the PR is updated with other commits, the list of reviewers is updated.
//
// Or, as a CI job progresses, it can be used to post links or information
// about the status of the BUILD (useful links, detected errors, etc).
//
// A naive BOT would just add new comments to a github PR, creating a poor
// user experience.
//
// StableComment will instead post one comment, and then keep updating it.
//
// The update operations support rendering a template with json, and allow
// for the state of the previous comment to be maintained.
//
// For example: let's say that a CI job is running. At the beginning of
// the run it creates a "StableComment", made by a template rendering
// a list of json operations.
//
// As the CI job continues, the "StableComment" API is invoked through
// independent CLI invocations (a "stateless" job - not a daemon),
// specifying a PATCH adding an operation to the previously posted
// json, causing operations to be added. The PATCH could look something
// like:
// 	[{"op":"add","path":"/operations/-","value":"{ ... json ...}"}]
// Appending an element to an existing list.
type StableComment struct {
	marker  string
	matcher *regexp.Regexp
	log     logger.Logger

	id          int64
	jsoncontent string
	template    string
}

type CommentPayload struct {
	Template string
	Content  string
}

// A unique string to ensure it's a comment added by this software.
// Note that a unique marker is also appended. Goats are probably enough here.
const kUniqueEnoughString = "A wise goat once said: "

type StableCommentModifier func(*StableComment) error

type StableCommentModifiers []StableCommentModifier

func (ms StableCommentModifiers) Apply(sc *StableComment) error {
	for _, mod := range ms {
		if err := mod(sc); err != nil {
			return err
		}
	}
	return nil
}

func WithMarker(marker string) StableCommentModifier {
	return func(sc *StableComment) error {
		sc.marker = marker
		return nil
	}
}

func WithTemplate(template string) StableCommentModifier {
	return func(sc *StableComment) error {
		sc.template = template
		return nil
	}
}

func WithLogger(logger logger.Logger) StableCommentModifier {
	return func(sc *StableComment) error {
		sc.log = logger
		return nil
	}
}

func WithJsonContent(content string) StableCommentModifier {
	return func(sc *StableComment) error {
		sc.jsoncontent = content
		return nil
	}
}

func WithID(id int64) StableCommentModifier {
	return func(sc *StableComment) error {
		sc.id = id
		return nil
	}
}

type StableCommentFlags struct {
	Marker      string
	Template    string
	JsonContent string
}

func DefaultStableCommentFlags() *StableCommentFlags {
	flags := &StableCommentFlags{
		Marker:      "staco-unfortunate-id",
		JsonContent: "{}",
	}
	return flags
}

func (fl *StableCommentFlags) Register(set kflags.FlagSet, prefix string) *StableCommentFlags {
	set.StringVar(&fl.Marker, prefix+"marker", fl.Marker, "A unique marker to identify the comment across subsequent runs of this command")
	set.StringVar(&fl.Template, prefix+"template", fl.Template, "Message to post in the comment, a text/template valorized through the json flag")
	set.StringVar(&fl.JsonContent, prefix+"json", fl.JsonContent, "JSON providing the default values for the text/template specified")
	return fl
}

type StableCommentDiffFlags struct {
	// Native jd format patch - as per https://github.com/josephburnett/jd#diff-language
	Diff string
	// RFC 7386 format.
	Patch string
	// RFC 6902 format.
	Merge string
}

func (fl *StableCommentDiffFlags) Register(set kflags.FlagSet, prefix string) *StableCommentDiffFlags {
	set.StringVar(&fl.Diff, prefix+"diff-jd", fl.Diff, "A change to apply in jd format - https://github.com/josephburnett/jd#diff-language")
	set.StringVar(&fl.Patch, prefix+"diff-patch", fl.Patch, "A change to apply in RFC 7386 format")
	set.StringVar(&fl.Merge, prefix+"diff-merge", fl.Patch, "A change to apply in RFC 6902 format")

	return fl
}

func NewDiffFromFlags(fl *StableCommentDiffFlags) (jd.Diff, error) {
	if (fl.Diff != "" && fl.Patch != "") || (fl.Diff != "" && fl.Merge != "") || (fl.Patch != "" && fl.Merge != "") {
		return nil, kflags.NewUsageErrorf("only one of --diff-jd, --diff-patch, and --diff-merge must be specified")
	}
	if fl.Diff == "" && fl.Patch == "" && fl.Merge == "" {
		return nil, kflags.NewUsageErrorf("one of --diff-jd, --diff-patch, and --diff-merge must be specified")
	}

	if fl.Diff != "" {
		return jd.ReadDiffString(fl.Diff)
	}
	if fl.Patch != "" {
		return jd.ReadPatchString(fl.Patch)
	}
	return jd.ReadMergeString(fl.Merge)
}

func NewStableComment(mods ...StableCommentModifier) (*StableComment, error) {
	sc := &StableComment{
		jsoncontent: "{}",
		marker:      "staco-unfortunate-id",
		log:         logger.Nil,
	}
	if err := StableCommentModifiers(mods).Apply(sc); err != nil {
		return nil, err
	}

	match, err := regexp.Compile("(?m)<!-- " + kUniqueEnoughString + regexp.QuoteMeta(sc.marker) + "\n(.*)\n-->")
	if err != nil {
		return nil, err
	}
	sc.matcher = match
	return sc, nil
}

func FromFlags(fl StableCommentFlags) StableCommentModifier {
	return func(sc *StableComment) error {
		if fl.Marker != "" {
			sc.marker = fl.Marker
		}
		sc.template = fl.Template
		sc.jsoncontent = fl.JsonContent

		return nil
	}
}

func (sc *StableComment) UpdateFromPR(rc *RepoClient, ctx context.Context, pr int) error {
	comments, err := rc.GetPRComments(ctx, pr)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		if comment.Body == nil || comment.ID == nil {
			continue
		}

		payload, template, err := sc.ParseComment(*comment.Body)
		if err != nil {
			// If there's a wrapped error, it means parsing json or templates failed.
			// Log the error, but otherwise re-use this comment. It was corrupted.
			if errors.Unwrap(err) == nil {
				continue
			}

			sc.log.Warnf("PR %d - Corrupted comment %d? %s", pr, *comment.ID, err)
		}

		sc.id = *comment.ID
		sc.jsoncontent = payload
		if sc.template == "" {
			sc.template = template
		}

		return nil
	}

	// NOT FOUND - no defaults.
	return nil
}

func (sc *StableComment) ParseComment(comment string) (string, string, error) {
	found := sc.matcher.FindStringSubmatch(comment)
	if len(found) < 2 {
		return "", "", fmt.Errorf("marker '%s' not found in:\n%s", sc.matcher, comment)
	}

	payload := CommentPayload{
		Content:  "{}",
		Template: "",
	}
	if found[1] != "" {
		if err := json.Unmarshal([]byte(found[1]), &payload); err != nil {
			return "", "", fmt.Errorf("invalid payload '%w' in:\n%s", err, payload)
		}
	}

	if err := json.Unmarshal([]byte(payload.Content), &map[string]interface{}{}); err != nil {
		return "", "", fmt.Errorf("invalid content payload '%w' in:\n%s", err, payload)
	}

	if _, err := template.New("template").Option("missingkey=error").Parse(payload.Template); err != nil {
		return "", "", fmt.Errorf("invalid template payload '%w' in:\n%s", err, payload)
	}

	return payload.Content, payload.Template, nil
}

func (sc *StableComment) PostPayload(rc *RepoClient, ctx context.Context, comment string, prnumber int) error {
	if sc.id == 0 {
		_, err := rc.AddPRComment(ctx, prnumber, comment)
		return err
	}

	return rc.EditPRComment(ctx, sc.id, comment)
}

func (sc *StableComment) PostToPR(rc *RepoClient, ctx context.Context, diff jd.Diff, prnumber int) error {
	payload, err := sc.PreparePayloadFromDiff(diff)
	if err != nil {
		return err
	}

	return sc.PostPayload(rc, ctx, payload, prnumber)
}

func (sc *StableComment) PreparePayloadFromDiff(diff jd.Diff) (string, error) {
	jc, err := jd.ReadJsonString(sc.jsoncontent)
	if err != nil {
		return "", err
	}
	jp, err := jc.Patch(diff)
	if err != nil {
		return "", err
	}

	return sc.PreparePayload(jp.Json())
}

func (sc *StableComment) PreparePayload(jsonvars string) (string, error) {
	vars := map[string]interface{}{}
	if err := json.Unmarshal([]byte(jsonvars), &vars); err != nil {
		return "", fmt.Errorf("invalid json supplied: %w -- '%s'", err, jsonvars)
	}

	tp, err := template.New("template").Option("missingkey=error").Parse(sc.template)
	if err != nil {
		return "", err
	}

	rendered := bytes.NewBufferString("")
	if err := tp.Execute(rendered, vars); err != nil {
		return "", err
	}

	payload := CommentPayload{
		Template: sc.template,
		Content:  jsonvars,
	}
	paystr, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return rendered.String() + "\n<!-- " + kUniqueEnoughString + sc.marker + "\n" + string(paystr) + "\n-->", nil
}
