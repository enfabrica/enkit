// This file provides a concrete parser for OWNERS file in the gerrit format.
//
// The gerrit format is "largely a superset" of what the CODEOWNERS file
// of github supports, so the parser here can actually be used for both,
// with minor tweaks.
//
// TODO(carlo): document/add missing tweaks. Main one: github CODEOWNERS
//   uses the last matching entry. Gerrit/current implementation merges
//   all matching entries, so all are applied.
//
// Unless you need to modify the parser behavior, you should only use the:
//
//    ParseGerritOwners(path string, data io.Reader) (*proto.Owners, error)
//
// function, which implements the parser.Reader interface.
//
// The grammar for all supported formats is defined below.
//
// Grammar defined for gerrit files:
// - https://gerrit.googlesource.com/plugins/find-owners/+/master/src/main/resources/Documentation/syntax.md
//
// Grammar defined for CODEOWNERS files:
// - https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners#codeowners-syntax
//
package parser

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/ownruler/proto"
)

type GerritOwners struct {
	parent bool
	*proto.Owners
}

type Location struct {
	File string
	Line int
}

func (l Location) String() string {
	if l.Line == 0 {
		return l.File
	}
	return fmt.Sprintf("%s:%d", l.File, l.Line)
}

func NewGerritOwners() *GerritOwners {
	return &GerritOwners{
		parent: true,
		Owners: &proto.Owners{},
	}
}

func (p *GerritOwners) AddConfig(data io.Reader, location Location) error {
	scanner := bufio.NewScanner(data)
	var errs []error
	for scanner.Scan() {
		location.Line += 1

		err := p.AddLine(scanner.Text(), location)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return multierror.New(errs)
}

var (
	reSingleUser = `file:\S+|\S*@\S+|[*]`

	reComment  = regexp.MustCompile(`#.*$`)
	reEmpty    = regexp.MustCompile(`^\s*$`)
	rePerFile  = regexp.MustCompile(`^per-file\s+(.*)$`)
	reInclude  = regexp.MustCompile(`^include\s+(.*)$`)
	reUser     = regexp.MustCompile(`^(` + reSingleUser + `)$`)
	rePattern  = regexp.MustCompile(`^(\S+)\s+((?:(?:` + reSingleUser + `)\s*)+)$`)
	reNoParent = regexp.MustCompile(`^set\s+noparent$`)
	reSpace    = regexp.MustCompile(`\s+`)
	reEqual    = regexp.MustCompile(`\s*=\s*`)
	reComma    = regexp.MustCompile(`\s*,\s*`)
)

func (p *GerritOwners) AddPerFileLine(perfile string, loc Location) error {
	statements := reEqual.Split(strings.TrimSpace(perfile), 2)
	if len(statements) != 2 {
		return fmt.Errorf("%s: expected 'pattern,pattern,...=user,user,...' but no '='?", loc)
	}

	pattern, user := statements[0], statements[1]

	if reNoParent.MatchString(user) {
		return p.AddReviewPatternParent(pattern, nil, false, loc)
	}

	patterns := reComma.Split(pattern, -1)
	if len(patterns) < 1 {
		return fmt.Errorf("%s: expected 'pattern,...=user,...' but no pattern?", loc)
	}

	var errs []error
	users := reComma.Split(user, -1)
	for _, pattern := range patterns {
		if err := p.AddReviewPattern(pattern, users, loc); err != nil {
			errs = append(errs, err)
		}
	}

	return multierror.New(errs)
}

func (p *GerritOwners) AddInclude(include string, loc Location) error {
	newaction := &proto.Action{
		Location: loc.String(),
		Op: &proto.Action_Include{
			include,
		},
	}
	p.Action = append(p.Action, newaction)
	return nil
}

func (p *GerritOwners) AddLine(data string, loc Location) error {
	// Remove commnets and leading / trailing whitespace.
	data = reComment.ReplaceAllString(data, "")
	data = strings.TrimSpace(data)

	if reEmpty.MatchString(data) {
		return nil
	}
	if matches := rePerFile.FindStringSubmatch(data); matches != nil {
		return p.AddPerFileLine(matches[1], loc)
	}
	if matches := reInclude.FindStringSubmatch(data); matches != nil {
		log.Printf("PROCESSING INCLUDE: %s", matches[1])
		return p.AddInclude(matches[1], loc)
	}
	if matches := reUser.FindStringSubmatch(data); matches != nil {
		return p.AddReviewPattern("", []string{matches[1]}, loc)
	}
	if matches := rePattern.FindStringSubmatch(data); matches != nil {
		return p.AddReviewPatternLine(matches[1], matches[2], loc)
	}
	if reNoParent.MatchString(data) {
		p.parent = false
		return nil
	}

	return fmt.Errorf("%s: meaningless line '%s'. To be considered a user, must have an '@' somewhere", loc, data)
}

func (p *GerritOwners) AddReviewPatternLine(pattern string, strusers string, loc Location) error {
	users := reSpace.Split(strusers, -1)
	return p.AddReviewPattern(pattern, users, loc)
}

func (p *GerritOwners) AddReviewPatternParent(match string, users []string, parent bool, loc Location) error {
	var errs []error
	for _, user := range users {
		if !reUser.MatchString(user) {
			errs = append(errs, fmt.Errorf("%s: invalid user '%s'", loc, user))
		}
	}
	if len(errs) > 0 {
		return multierror.New(errs)
	}

	var tochange *proto.Action
	for _, action := range p.Action {
		switch at := action.Op.(type) {
		case *proto.Action_Review:
			if at.Review.Pattern == match {
				tochange = action
				break
			}
		default:
		}
	}

	if tochange == nil {
		tochange = &proto.Action{
			Location: loc.String(),
			Op: &proto.Action_Review{
				&proto.Match{
					Pattern: match,
					Parent:  parent,
				},
			},
		}
		p.Action = append(p.Action, tochange)
	}

	review := tochange.Op.(*proto.Action_Review).Review
	review.User = mergeUsers(review.User, users, loc)
	return nil
}

func (p *GerritOwners) AddReviewPattern(match string, users []string, loc Location) error {
	return p.AddReviewPatternParent(match, users, p.parent, loc)
}

func mergeUsers(dest []*proto.User, src []string, loc Location) []*proto.User {
	resulting := []*proto.User{}
	existing := map[string]struct{}{}

	for _, u := range dest {
		_, found := existing[u.Identifier]
		if found {
			continue
		}
		existing[u.Identifier] = struct{}{}
		resulting = append(resulting, u)
	}
	for _, u := range src {
		_, found := existing[u]
		if found {
			continue
		}
		existing[u] = struct{}{}
		resulting = append(resulting, &proto.User{
			Identifier: u,
			Location:   loc.String(),
		})
	}

	return resulting
}

func ParseGerritOwners(path string, data io.Reader) (*proto.Owners, error) {
	location := Location{File: path, Line: 0}
	owners := NewGerritOwners()
	if err := owners.AddConfig(data, location); err != nil {
		return nil, err
	}
	owners.Owners.Location = path
	return owners.Owners, nil
}
