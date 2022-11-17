package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/josephburnett/jd/lib"
        "github.com/Masterminds/sprig/v3"
        "github.com/itchyny/gojq"
	"os"
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
	jsonreset   bool
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

func WithJsonReset(reset bool) StableCommentModifier {
	return func(sc *StableComment) error {
		sc.jsonreset = reset
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
	JsonReset   bool
}

var DefaultMarker = "staco-unfortunate-id"

func DefaultStableCommentFlags() *StableCommentFlags {
	flags := &StableCommentFlags{
		Marker:      DefaultMarker,
		JsonContent: "{}",
	}
	return flags
}

func (fl *StableCommentFlags) Register(set kflags.FlagSet, prefix string) *StableCommentFlags {
	set.StringVar(&fl.Marker, prefix+"marker", fl.Marker, "A unique marker to identify the comment across subsequent runs of this command")
	set.StringVar(&fl.Template, prefix+"template", fl.Template, "Message to post in the comment, a text/template valorized through the json flag")
	set.StringVar(&fl.JsonContent, prefix+"json", fl.JsonContent, "JSON providing the default values for the text/template specified")
	set.BoolVar(&fl.JsonReset, prefix+"reset", fl.JsonReset, "Ignore the JSON parsed from the PR, start over with the default json in --json")
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

func DefaultStableCommentDiffFlags() *StableCommentDiffFlags {
	return &StableCommentDiffFlags{}
}

func (fl *StableCommentDiffFlags) Register(set kflags.FlagSet, prefix string) *StableCommentDiffFlags {
	set.StringVar(&fl.Diff, prefix+"diff-jd", fl.Diff, "A change to apply in jd format - https://github.com/josephburnett/jd#diff-language")
	set.StringVar(&fl.Patch, prefix+"diff-patch", fl.Patch, "A change to apply in RFC 7386 format (patch format)")
	set.StringVar(&fl.Merge, prefix+"diff-merge", fl.Patch, "A change to apply in RFC 6902 format (merge format)")

	return fl
}

func NewDiffFromFlags(fl *StableCommentDiffFlags) (jd.Diff, error) {
	if (fl.Diff != "" && fl.Patch != "") || (fl.Diff != "" && fl.Merge != "") || (fl.Patch != "" && fl.Merge != "") {
		return nil, kflags.NewUsageErrorf("only one of --diff-jd, --diff-patch, and --diff-merge must be specified")
	}

	if fl.Diff != "" {
		return jd.ReadDiffString(fl.Diff)
	}
	if fl.Patch != "" {
		return jd.ReadPatchString(fl.Patch)
	}
	if fl.Merge != "" {
		return jd.ReadMergeString(fl.Merge)
	}
	return nil, nil
}

type DiffTransformer jd.Diff

func (dt *DiffTransformer) Apply(ijson string) (string, error) {
	jc, err := jd.ReadJsonString(ijson)
	if err != nil {
		return "", err
	}
	if dt == nil {
		return jc.Json(), nil
	}

	jp, err := jc.Patch(jd.Diff(*dt))
	if err != nil {
		return "", err
	}

	return jp.Json(), nil
}

type StableCommentJqFlags struct {
	Timeout time.Duration
	Code string
}

func DefaultStableCommentJqFlags() *StableCommentJqFlags {
	return &StableCommentJqFlags{
		Timeout: time.Second * 1,
	}
}

func (fl *StableCommentJqFlags) Register(set kflags.FlagSet, prefix string) *StableCommentJqFlags {
	set.DurationVar(&fl.Timeout, prefix+"jq-timeout", fl.Timeout, "How long to wait at most for the jq program to terminate")
	set.StringVar(&fl.Code, prefix+"jq-code", fl.Code, "The actual jq program to run")
	return fl
}

type JqTransformer struct {
	code *gojq.Code
	timeout time.Duration
}

func (jt *JqTransformer) Apply(ijson string) (string, error) {
	if jt == nil || jt.code == nil {
		return ijson, nil
	}

	var pjson map[string]interface{}
	if err := json.Unmarshal([]byte(ijson), &pjson); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), jt.timeout)
	defer cancel()

	results := jt.code.RunWithContext(ctx, pjson)

	first, ok := results.Next()
	if !ok {
		return "", fmt.Errorf("jq script returned no value")
	}
	if err, ok := first.(error); ok {
		return "", fmt.Errorf("jq script execution returned error - %w", err)
	}

	second, ok := results.Next()
	if ok {
		return "", fmt.Errorf("jq script returned too many values - %#v", second)
	}

	omap, ok := first.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("jq script returned something that's not a map[] - %#v", first)
	}
	ojson, err := json.Marshal(omap)
	if err != nil {
		return "", fmt.Errorf("jq returned something that cannot be marshalled - %#v", omap)
	}
	return string(ojson), nil
}

func NewJqFromFlags(jqf *StableCommentJqFlags) (*JqTransformer, error) {
	if jqf.Code == "" {
		return nil, nil
	}
	q, err := gojq.Parse(jqf.Code)
	if err != nil {
		return nil, err
	}

	code, err := gojq.Compile(q, gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return nil, err
	}
	return &JqTransformer{code: code, timeout: jqf.Timeout}, nil
}

type Transformer interface {
	Apply(inputjson string) (string, error)
}

type StableCommentTransformerFlags struct {
	jqFlags *StableCommentJqFlags
	diffFlags *StableCommentDiffFlags
}

func DefaultStableCommentTransformerFlags() *StableCommentTransformerFlags {
	return &StableCommentTransformerFlags{
		jqFlags: DefaultStableCommentJqFlags(),
		diffFlags: DefaultStableCommentDiffFlags(),
	}
}

func (fl *StableCommentTransformerFlags) Register(
    set kflags.FlagSet, prefix string) *StableCommentTransformerFlags {
	fl.jqFlags.Register(set, prefix)
	fl.diffFlags.Register(set, prefix)
	return fl
}

func NewTransformerFromFlags(fl *StableCommentTransformerFlags) (Transformer, error) {
	jq, err := NewJqFromFlags(fl.jqFlags)
	if err != nil {
		return nil, err
	}

	diff, err := NewDiffFromFlags(fl.diffFlags)
	if err != nil {
		return nil, err
	}

	if jq != nil {
		return jq, nil
	}
	return (*DiffTransformer)(&diff), nil
}

func NewStableComment(mods ...StableCommentModifier) (*StableComment, error) {
	sc := &StableComment{
		jsoncontent: "{}",
		marker:      DefaultMarker,
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

func StableCommentFromFlags(fl *StableCommentFlags) StableCommentModifier {
	return func(sc *StableComment) error {
		if fl.Marker != "" {
			sc.marker = fl.Marker
		}
		sc.template = fl.Template
		sc.jsoncontent = fl.JsonContent
		sc.jsonreset = fl.JsonReset

		return nil
	}
}

func (sc *StableComment) UpdateFromPR(rc *RepoClient, pr int) error {
	id, payload, template, err := sc.FetchPRState(rc, pr)
	if err != nil {
		return err
	}

	sc.id = id
	if payload != "" && !sc.jsonreset {
		sc.jsoncontent = payload
	}
	if sc.template == "" {
		sc.template = template
	}
	return nil
}

func (sc *StableComment) FetchPRState(rc *RepoClient, pr int) (int64, string, string, error) {
	comments, err := rc.GetPRComments(pr)
	if err != nil {
		return 0, "", "", err
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
		return *comment.ID, payload, template, nil
	}

	// NOT FOUND - no defaults.
	return 0, "", "", nil
}

// ParseComment parses a string comment.
//
// The string comment is normally retrieved from github,
// but this function can be used for scripts or tests.
//
// Returns the parsed and validated json content and template.
//
// If the error returns nil on Unwrap() it means there was
// no parsing or validation error - the supplied string
// did not contain metadata.
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

	if _, err := template.New("template").Funcs(sprig.FuncMap()).Option("missingkey=error").Parse(payload.Template); err != nil {
		return "", "", fmt.Errorf("invalid template payload '%w' in:\n%s", err, payload)
	}

	return payload.Content, payload.Template, nil
}

// PostPayload posts a pre-formatted comment to the specified PR.
func (sc *StableComment) PostPayload(rc *RepoClient, comment string, prnumber int) error {
	if sc.id == 0 {
		_, err := rc.AddPRComment(prnumber, comment)
		return err
	}

	return rc.EditPRComment(sc.id, comment)
}

// PostAction describes the action that needs to be performed for this comment.
func (sc *StableComment) PostAction() string {
	if sc.id == 0 {
		return "create new comment"
	}
	return fmt.Sprintf("edit comment ID %d", sc.id)
}

func (sc *StableComment) PostToPR(rc *RepoClient, tr Transformer, prnumber int) error {
	payload, err := sc.PreparePayloadFromDiff(tr)
	if err != nil {
		return err
	}

	return sc.PostPayload(rc, payload, prnumber)
}

func (sc *StableComment) PreparePayloadFromDiff(tr Transformer) (string, error) {
	ojson, err := tr.Apply(sc.jsoncontent)
	if err != nil {
		return "", err
	}

	return sc.PreparePayload(ojson)
}

// PreparePayload prepares a comment to post based on the specified jsonvars.
//
// jsonvars is a json payload, as a string.
//
// Returns the payload, ready to be posted with PostPayload(), or an error.
func (sc *StableComment) PreparePayload(jsonvars string) (string, error) {
	vars := map[string]interface{}{}
	if err := json.Unmarshal([]byte(jsonvars), &vars); err != nil {
		return "", fmt.Errorf("invalid json supplied: %w -- '%s'", err, jsonvars)
	}

	tp, err := template.New("template").Funcs(sprig.FuncMap()).Option("missingkey=error").Parse(sc.template)
	if err != nil {
		return "", err
	}

	rendered := bytes.NewBufferString("")
	if err := tp.Execute(rendered, vars); err != nil {
		return "", fmt.Errorf("template expansion failed! Template:\n%s\nVariables:\n%s\nError: %w", sc.template, jsonvars, err)
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
