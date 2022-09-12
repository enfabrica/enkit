package parser

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/ownruler/proto"
	"github.com/go-git/go-billy"
	"github.com/go-git/go-billy/util"
	"github.com/go-git/go-git/plumbing/format/gitignore"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Reader is a function that given the path to a file and a reader
// returning its content, it parses the file and returns a protocol
// buffer describing a set of Owners.
//
// Note that the path here is just a string: the Reader should not
// expect to be able to find the file or Stat it (it may be
// in memory, remote via http, ...). The only thing it can do is
// parse its content via the supplied io.Reader.
type Reader func(path string, data io.Reader) (*proto.Owners, error)

// Loader binds a pattern (example: "*.codeowners") to a Reader.
//
// When scanning a directory tree with multiple owners file, each
// file will be parsed by the first Reader matching the Pattern,
// see the Load() function below.
//
// Matching is based on filepath.Match, eg, fnmatch/glob style matches.
type Loader struct {
	Pattern string
	Reader  Reader
}

// Loaders represents a list of Loader.
type Loaders []Loader

// Load parses a file with the first Reader matching its path.
//
// The path of the file is treated just like a string identifier,
// may or may not correspond to a file on the filesystem (may be in memory,
// remote, ...). The file is read through the input io.Reader.
//
// Returns an error if no Loader can be file matching the name of the file.
func (h Loaders) Load(path string, input io.Reader) (*proto.Owners, error) {
	filename := filepath.Base(path)
	for _, loader := range h {
		match, err := filepath.Match(loader.Pattern, filename)
		if err == nil && match {
			return loader.Reader(path, input)
		}
	}

	patterns := []string{}
	for _, loader := range h {
		patterns = append(patterns, loader.Pattern)
	}

	// TODO(carlo): define a specific error? Wrap something that represents ENOENT?
	return nil, fmt.Errorf("do not know how to parse '%s' - name must match one of %v", path, patterns)
}

// LoadFS opens and parses a file from the FileSystem.
//
// Just like Load, LoadFS parses an OWNERS file with the first
// Reader matching the path of the file.
//
// Instead of expecting an io.Reader, LoadFS opens the file via
// a billy.Filesystem object.
func (h Loaders) LoadFS(fs billy.Filesystem, path string) (*proto.Owners, error) {
	input, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	pb, err := h.Load(path, input)
	if err != nil {
		return nil, err
	}
	return pb, nil
}

// Patterns represents a list of filepath.Match patterns.
type Patterns []string

// Valid verifies that a list of patterns is valid.
//
// Just like with regular expressions, some glob patterns
// are invalid/cannot be used.
//
// Valid() goes through a list of patterns, and returns
// error if any of them is invalid.
func (p Patterns) Valid() error {
	for _, pattern := range p {
		_, err := filepath.Match(pattern, "")
		if err != nil {
			return err
		}
	}
	return nil
}

// Match returns true if the supplied string matches any pattern.
func (p Patterns) Match(name string) bool {
	for _, pattern := range p {
		match, err := filepath.Match(pattern, name)
		if err == nil && match {
			return true
		}
	}
	return false
}

// GitPattern represents a .gitignore style pattern.
//
// .gitignore style pattern are an extended form of glob/fnmatch
// patterns, as they support syntax like **/ to represent any
// number of subdirectories, or negative matches.
//
// Simple strings with no "/" also have a slightly different
// meaning, as they generally refer to a file within the current
// directory, and not a random string part of the file name.
//
// In order to work, GitPatterns need to know in which directory
// they were defined, and the elements of the path.
type GitPattern struct {
	gitignore.Pattern

	// The directory where the pattern was defined.
	dir     string
	// The original string defining the pattern.
	pattern string

	// The directory where the pattern was defined, broken
	// down by '/' in individual elements.
	//
	// This is needed by the gitignore.Pattern library.
	els     []string
}

// NewGitPattern creates a new GitPattern object.
func NewGitPattern(patterndir, pattern string) *GitPattern {
	var els []string
	if len(patterndir) > 0 {
		els = strings.Split(patterndir, "/")
	}
	if pattern == "" {
		pattern = "*"
	}
	return &GitPattern{Pattern: gitignore.ParsePattern(pattern, els), dir: patterndir, pattern: pattern, els: els}
}

// Match checks if a specific file path matches a GitPattern.
//
// Returns true if it does.
func (p GitPattern) Match(tomatch string) bool {
	els := strings.Split(tomatch, "/")
	result := p.Pattern.Match(els, false)
	if result == gitignore.NoMatch || result == gitignore.Include {
		return false
	}
	return true
}

// DefaultLoaders define the default patterns and corresponding readers.
var DefaultLoaders = Loaders{
	// TODO(carlo): ParseGerritOwners should be extended to have an
	// option so the last matching entry is used, mimicing the github
	// behavior. As is, it can parse a CODEOWNERS file correctly, use
	// it, but instead of picking the last owner like github, all
	// matching owners are used!
	Loader{Pattern: "CODEOWNERS", Reader: ParseGerritOwners},

	Loader{Pattern: "OWNERS", Reader: ParseGerritOwners},
	Loader{Pattern: "METADATA", Reader: ParseProtoOwners},

	Loader{Pattern: "*.codeowners", Reader: ParseGerritOwners},
	Loader{Pattern: "*.owners", Reader: ParseGerritOwners},
	Loader{Pattern: "*.metadata", Reader: ParseProtoOwners},
}

// DefaultIndexes defines the files to look for in a directory.
//
// For example: if this library is asked to find the OWNERS for
// the file "source/code/file.c", the code will try to find any
// file matching the patterns here in "source/code/", "source/",
// and the root of the repository.
var DefaultIndexes = Patterns{
	"CODEOWNERS",
	"OWNERS",
	"METADATA",
}

// OnError is a generic function that taken a path and an error
// processing that path, decides what to do next.
//
// If error is returned, the scan is stopped, and the corresponding
// error is returned.
// If nil is returned, the scan continues, assuming that OnError
// took care of reporting or addressing the problem.
type OnError func(path string, err error) error

// Owners object is an index of files, directories, and corresponding Owners.
//
// Once loaded/created, it can be queried without having any of the
// corresponding files or directories available anymore. It would be
// possible to store an Owners() object in a {key, value} databse,
// and query it with some of its functions.
//
// Creating or using an Owners object is not necessary, however:
// the actions associated to a file or directory can be determined
// directly using the Actions() function below.
//
// It's however important to note that if the OWNERS files in
// a directory tree contain errors, Actions() will not detect
// those errors until it is used for a path that references
// those OWNERS files.
//
// Creating an Owners index will instead detect all errors at
// load time, and may be a good way to validate the correctness
// of OWNERS files in a tree.
type Owners struct {
	files map[string]*proto.Owners
	dirs  map[string]*proto.Owners
}

// NewOwners creates a new empty index.
func NewOwners() *Owners {
	owners := &Owners{
		files: map[string]*proto.Owners{},
		dirs:  map[string]*proto.Owners{},
	}

	return owners
}

// Add binds a path representing a file to a specific config.
func (owners *Owners) Add(path string, pb *proto.Owners) {
	owners.files[path] = pb

	dirname := filepath.Dir(path)
	of := owners.dirs[dirname]
	if of == nil {
		owners.dirs[dirname] = pb
	} else {
		// Why replace? It's a pointer, that may be referenced from other structs.
		//
		// To modify it while guaranteeing that other structs are not affected,
		// we must make a new copy.
		replacement := &proto.Owners{
			Location: of.Location,
		}
		replacement.Action = append(replacement.Action, of.Action...)
		replacement.Action = append(replacement.Action, pb.Action...)

		owners.dirs[dirname] = replacement
	}
}

// AbsPath computes a path absolute to the root of the repository.
//
// Similar to filepath.Join(), AbsPath joins multiple elements of
// a path together. But if any of those elements starts with '/',
// that element is considered absolute to the root of the repository.
//
// See ExampleAbsPath for an example.
func AbsPath(root string, components ...string) string {
	start := len(components) - 1
	for ; start > 0 && !filepath.IsAbs(components[start]); start-- {
	}

	return filepath.Join(append([]string{root}, components[start:]...)...)
}

// RelPath joins a child path with its parent directory.
//
// RelPath is normally used to return a path relative to the root of the
// repository, and is assumed to be passed two paths within the repository.
//
// Underneath, it just invokes filepath.Join(base, child) unless the child path
// starts with '/', in which case it is assumed to be a direct desendent
// of the root of the repository, and the path is returned instead
// (with the leading / stripped, to prevent escaping the root of the repo).
func RelPath(base, child string) string {
	if filepath.IsAbs(child) {
		return child[1:]
	}

	return filepath.Join(base, child)
}

// UsernameIsFile returns a file coded in an username.
//
// Gerrit syntax allows for an OWNERS file to have file:/path/of/another/OWNERS
// in place of an username. This allows, for example, to expand a line with
// a group of users, read from another directory.
//
// This functions takes as input an username, and returns the path of the
// file if the username represents a file, or an empty string if it doesn't.
func UsernameIsFile(username string) string {
	upath := strings.TrimPrefix(username, "file:")
	if upath == username {
		return ""
	}
	return upath
}

// Dependencies returns the dependencies of a proto.Owners.
//
// OWNERS files can have 'include' statements loading other
// OWNERS files, or 'file:' statements in place of usernames.
//
// Given a protocol buffer representing an OWNERS file, this
// function returns its full set of dependencies: all the other
// files it references - regardless of where they are coming
// from.
//
// This is useful for validation or indexing.
//
// The paths returned are the strings specified in the protocol
// buffer itself, without any sort of validation or normalization.
func Dependencies(pb *proto.Owners) []string {
	deps := []string{}
	for _, action := range pb.Action {
		var match *proto.Match
		switch at := action.Op.(type) {
		case *proto.Action_Include:
			deps = append(deps, at.Include)
			continue

		case *proto.Action_Review:
			match = at.Review
		case *proto.Action_Notify:
			match = at.Notify

		default:
			continue
		}

		for _, user := range match.User {
			upath := UsernameIsFile(user.Identifier)
			if upath == "" {
				continue
			}
			deps = append(deps, upath)
		}
	}
	return deps
}

// Preload loads all dependencies of added paths.
//
// The Preload() function should ALWAYS be called before using an
// Owners object. In short, it checks all the paths and owners configs
// added with Add(), and ensures that all the other files they reference
// are actually parsed and loaded. If not, it uses the supplied Filesystem
// object and set of Loaders to load them.
//
// This is necessary as OWNERS files can reference other OWNERS files
// that may (or may not) be the index of any directory. For example:
//   /project/enkit/OWNERS may reference /project/contributors.owners
//   as well as /project/enkit/lib/OWNERS.
//
// Based on the Indexes config, Scan() below would load /project/enkit/OWNERS,
// /project/enkit/lib/OWNERS, but not /project/contibutors.owners.
//
// Also, there is no way to know if any dependency is missing until
// all have been Add()ed.
//
// TODO(carlo): stop using billy.Filesystem directly, do like Actions().
func (owners *Owners) Preload(fs billy.Filesystem, root string, loaders Loaders) error {
	type item struct {
		path string
		pb   *proto.Owners
	}

	to_process := []item{}
	for path, pb := range owners.dirs {
		to_process = append(to_process, item{
			path: path,
			pb:   pb,
		})
	}

	var errs []error
	for len(to_process) > 0 {
		last := to_process[len(to_process)-1]
		to_process = to_process[:len(to_process)-1]

		deps := Dependencies(last.pb)
		for _, dep := range deps {
			ipath := RelPath(last.path, dep)
			if owners.files[ipath] != nil {
				continue
			}

			fullpath := AbsPath(root, last.path, dep)
			pb, err := loaders.LoadFS(fs, fullpath)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: could not load %s - %w", last.pb.Location, fullpath, err))
				continue
			}

			owners.files[ipath] = pb
			to_process = append(to_process, item{path: ipath, pb: pb})
		}
	}
	return multierror.New(errs)
}

// ScanConfig configures a Scan.
type ScanConfig struct {
	// Indexes to use. See description for DefaultIndexes.
	Indexes Patterns
	// Loaders to use. See description for Loaders.
	Loaders Loaders
	// What to do on error. See description for OnError.
	OnError OnError
}

// Scan adds all the index owners file in a tree.
//
// Scan reads an entire repository, one directory at a time. For each directory,
// it checks if an "index" OWNERS file is found. If it's there, it calls Add()
// to add it.
//
// root is the root of the repository, while config provides the list of
// patterns to match to find index files, and the list of loaders to use
// depending on the filename.
//
// Invokes Preload() at the end of the Scan to return a fully usable
// Owners() index.
//
// TODO(carlo): stop using billy.Filesystem directly, do like Actions().
func (owners *Owners) Scan(fs billy.Filesystem, root string, config ScanConfig) (*Owners, error) {
	var errs []error
	if err := util.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if !config.Indexes.Match(info.Name()) {
			return nil
		}

		pb, err := config.Loaders.LoadFS(fs, AbsPath(root, path))
		if err != nil {
			if config.OnError != nil {
				return config.OnError(path, err)
			}
			return err
		}

		owners.Add(path, pb)
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	if err := owners.Preload(fs, root, config.Loaders); err != nil {
		errs = append(errs, err)
	}
	return owners, multierror.New(errs)
}

// GetDir returns the owners for a directory.
func (owners *Owners) GetDir(dir string) (*proto.Owners, error) {
	return owners.dirs[dir], nil
}

// GetDir returns the owners definition in a specific file.
func (owners *Owners) GetFile(path string) (*proto.Owners, error) {
	return owners.files[path], nil
}

// Actions determines the actions for a specific file.
//
// This wraps the free standing Actions() below so that all the information
// it needs to determine the actions for a file are retrieved from the
// generated index. 
func (owners *Owners) Actions(file string) ([]proto.User, []proto.User, error) {
	return Actions(file, owners.GetDir, owners.GetFile)
}

// A FileOpener returns a *proto.Owners for the exact path specified.
type FileOpener func(filename string) (*proto.Owners, error)

// A DirOpener searches for an OWNER file in a directory, and returns a
// *proto.Owners if found.
type DirOpener func(dirname string) (*proto.Owners, error)

// Matcher maintains the state to match a file.
type Matcher struct {
	file string
	open FileOpener

	reviewSeen map[string]struct{}
	notifySeen map[string]struct{}
}

// NewMatcher creates a new Matcher.
func NewMatcher(file string, opener FileOpener) *Matcher {
	return &Matcher{
		file:       file,
		open:       opener,
		reviewSeen: map[string]struct{}{},
		notifySeen: map[string]struct{}{},
	}
}

// Match returns matching entries in the OWNERS file in the opath directory.
//
// Match will check if the file path supplied to NewMatcher() matches any action
// in the proto.Owners supplied as target, and readable from the directory opath
// (opath is the directory of the OWNERS file, must be the path to a directory).
//
// It returns the list of users marked as reviewers, the list of users to
// notify, and wether the OWNERS file in the parent directory should be checked.
//
// Matcher.Match is a building block to implement recursive OWNERS file checking
// without loops.
func (matcher *Matcher) Match(opath string, target *proto.Owners) ([]proto.User, []proto.User, bool, error) {
	revs := []proto.User{}
	nots := []proto.User{}
	var ltarget *[]proto.User
	var dtarget *map[string]struct{}

	var match *proto.Match
	var errs []error

	parent := true
	recurse := func(location, upath string) ([]proto.User, []proto.User, bool) {
		log.Printf("RECURSING %s - %s", location, upath)
		upath = RelPath(opath, upath)
		sub, err := matcher.open(upath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s references %s, which could not be opened: %w", location, upath, err))
			return nil, nil, true
		}

		dpath := filepath.Dir(upath)
		urevs, unots, uparent, err := matcher.Match(dpath, sub)
		if err != nil {
			errs = append(errs, err)
		}
		log.Printf("FOUND %#v - %#v", urevs, uparent)
		return urevs, unots, uparent
	}

	for _, action := range target.Action {
		location := action.Location
		if location == "" {
			location = target.Location
		}

		switch at := action.Op.(type) {
		case *proto.Action_Include:
			urevs, unots, uparent := recurse(location, at.Include)
			revs = append(revs, urevs...)
			nots = append(nots, unots...)
			parent = parent && uparent
			continue

		case *proto.Action_Review:
			ltarget = &revs
			dtarget = &matcher.reviewSeen
			match = at.Review

		case *proto.Action_Notify:
			ltarget = &nots
			dtarget = &matcher.notifySeen
			match = at.Notify
		}

		matches := NewGitPattern(opath, match.Pattern).Match(matcher.file)
		if !matches {
			continue
		}

		if match.Location != "" {
			location = match.Location
		}
		if !match.Parent {
			// empty Pattern are default global entries, like:
			//     per-file *.py @tony
			//     @denny <<<-- This has Pattern == ""
			//
			// A no parent in a default entry should be ignored if there
			// were more specific matches before that did not have no-parent
			// (specific matches no-parent will still be honored).
			if match.Pattern != "" || len(*ltarget) <= 0 {
				parent = false
			}
		}

		for _, user := range match.User {
			if _, found := (*dtarget)[user.Identifier]; found {
				continue
			}
			(*dtarget)[user.Identifier] = struct{}{}

			upath := UsernameIsFile(user.Identifier)
			if upath == "" {
				toadd := *user
				if toadd.Location == "" {
					toadd.Location = location
				}

				(*ltarget) = append(*ltarget, toadd)
			} else {
				urevs, unots, _ := recurse(location, upath)
				var source []proto.User
				if ltarget == &revs {
					source = urevs
				} else {
					source = unots
				}

				// We are guaranteed source not to contain duplicates as the
				// recursive match uses the same hash table to prevent duplicates.
				(*ltarget) = append(*ltarget, source...)
			}
		}
	}
	return revs, nots, parent, multierror.New(errs)
}

// FsFileOpener returns a FileOpener capable of loading files from a FileSystem.
func FsFileOpener(fs billy.Filesystem, loaders Loaders) FileOpener {
	return func(file string) (*proto.Owners, error) {
		proto, err := loaders.LoadFS(fs, file)
		log.Printf("Loading file: %s - %#v, %#v", file, proto, err)
		return proto, err
	}
}

// FsDirOpener returns a DirOpener capable of loading an index from a directory.
func FsDirOpener(fs billy.Filesystem, indexes Patterns, loaders Loaders) DirOpener {
	return func(dir string) (*proto.Owners, error) {
		files, err := fs.ReadDir(dir)
		if err != nil {
			return nil, err
		}

		var config *proto.Owners
		var errs []error
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if !indexes.Match(file.Name()) {
				continue
			}

			pb, err := loaders.LoadFS(fs, filepath.Join(dir, file.Name()))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if config == nil {
				config = &proto.Owners{
					Location: pb.Location,
				}
			}
			config.Action = append(config.Action, pb.Action...)
		}

		log.Printf("Loading dir: %s - %#v, %s", dir, config, multierror.New(errs))
		return config, multierror.New(errs)
	}
}

// Actions determines the actions to be performed for the specified file.
//
// Actions() uses the DirOpener and FileOpener specified to find all
// the OWNERS/METADATA/... files that apply to "file", and return
// the corresponding actions (mandatory reviewers, users to be notified).
func Actions(file string, dopen DirOpener, fopen FileOpener) ([]proto.User, []proto.User, error) {
	left := filepath.Clean(file)
	right := ""

	revs := []proto.User{}
	nots := []proto.User{}

	revd := map[string]struct{}{}
	notd := map[string]struct{}{}

	add := func(toadd []proto.User, seen map[string]struct{}, target *[]proto.User) {
		for _, user := range toadd {
			if _, found := seen[user.Identifier]; found {
				continue
			}
			seen[user.Identifier] = struct{}{}
			(*target) = append(*target, user)
		}
	}

	var errs []error
	for len(left) > 0 {
		dir, base := filepath.Split(left)
		dir = strings.TrimSuffix(dir, "/")

		config, err := dopen(dir)
		if err != nil {
			errs = append(errs, err)
		} else if config != nil {
			review, notify, parent, err := NewMatcher(file, fopen).Match(dir, config)
			if err != nil {
				errs = append(errs, err)
			}

			add(review, revd, &revs)
			add(notify, notd, &nots)

			if !parent {
				break
			}
		}

		left = dir
		right = filepath.Join(base, right)
	}
	return revs, nots, multierror.New(errs)
}
