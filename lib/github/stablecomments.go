package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/josephburnett/jd/lib"
	"log"
	"regexp"
	"text/template"
)

type StaticComment struct {
	marker  string
	matcher *regexp.Regexp

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

type StaticCommentModifier func(*StaticComment) error

type StaticCommentModifiers []StaticCommentModifier

func (ms StaticCommentModifiers) Apply(sc *StaticComment) error {
	for _, mod := range ms {
		if err := mod(sc); err != nil {
			return err
		}
	}
	return nil
}

func WithTemplate(template string) StaticCommentModifier {
	return func(sc *StaticComment) error {
		sc.template = template
		return nil
	}
}

func WithJsonContent(content string) StaticCommentModifier {
	return func(sc *StaticComment) error {
		sc.jsoncontent = content
		return nil
	}
}

func WithID(id int64) StaticCommentModifier {
	return func(sc *StaticComment) error {
		sc.id = id
		return nil
	}
}

func NewStaticComment(marker string, mods ...StaticCommentModifier) (*StaticComment, error) {
	sc := &StaticComment{
		jsoncontent: "{}",
		marker:      marker,
	}
	if err := StaticCommentModifiers(mods).Apply(sc); err != nil {
		return nil, err
	}

	match, err := regexp.Compile("(?m)<!-- " + kUniqueEnoughString + regexp.QuoteMeta(marker) + "\n(.*)\n-->")
	if err != nil {
		return nil, err
	}
	sc.matcher = match
	return sc, nil
}

func (sc *StaticComment) UpdateFromPR(rc *RepoClient, ctx context.Context, pr int) error {
	comments, err := rc.GetPRComments(ctx, pr)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		log.Printf("GOT %s", comment)

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

			log.Printf("Comment was corrupted? %s", err)
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

func (sc *StaticComment) ParseComment(comment string) (string, string, error) {
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

func (sc *StaticComment) PostPayload(rc *RepoClient, ctx context.Context, comment string, prnumber int) error {
	if sc.id == 0 {
		return rc.AddPRComment(ctx, prnumber, comment)
	}

	return rc.EditPRComment(ctx, sc.id, comment)
}

func (sc *StaticComment) PostToPR(rc *RepoClient, ctx context.Context, diff jd.Diff, prnumber int) error {
	payload, err := sc.PreparePayloadFromDiff(diff)
	if err != nil {
		return err
	}

	return sc.PostPayload(rc, ctx, payload, prnumber)
}

func (sc *StaticComment) PreparePayloadFromDiff(diff jd.Diff) (string, error) {
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

func (sc *StaticComment) PreparePayload(jsonvars string) (string, error) {
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
