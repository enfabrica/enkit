package bazel

import (
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"sort"
	"strconv"
	"strings"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
)

// Target wraps a build.proto Target message with some lazily computed
// properties.
type Target struct {
	*bpb.Target

	// Memoized hash of this target. If nil, the hash is not computed yet; use
	// getHash() to fetch a computed hash.
	hash *uint32
	// If this target is a rule with dependencies, direct dependency node pointers
	// are stored here for easy traversal.
	deps []*Target
}

type TargetHashes map[string]uint32

// Name returns the name of a Target message, which is part of a pseudo-union
// message (enum + one populated optional field).
func (t *Target) Name() string {
	switch t.Target.GetType() {
	case bpb.Target_RULE:
		return t.Target.GetRule().GetName()
	case bpb.Target_SOURCE_FILE:
		return t.Target.GetSourceFile().GetName()
	case bpb.Target_GENERATED_FILE:
		return t.Target.GetGeneratedFile().GetName()
	case bpb.Target_PACKAGE_GROUP:
		return t.Target.GetPackageGroup().GetName()
	case bpb.Target_ENVIRONMENT_GROUP:
		return t.Target.GetEnvironmentGroup().GetName()
	}
	// This shouldn't happen; check that all cases are covered.
	panic(fmt.Sprintf("can't get name for type %q", t.Target.GetType()))
}

func (t *Target) ruleType() string {
	if t.Target.GetType() != bpb.Target_RULE {
		return ""
	}
	return t.Target.GetRule().GetRuleClass()
}

func (t *Target) containsTag(tag string) bool {
	attrList := t.Target.GetRule().GetAttribute()
	for _, attr := range attrList {
		if attr.GetName() != "tags" {
			continue
		}
		if attr.GetType() != bpb.Attribute_STRING_LIST {
			continue
		}
		for _, t := range attr.GetStringListValue() {
			if t == tag {
				return true
			}
		}
	}
	return false
}

// getHash returns the computed hash from this target, recursively evaluating
// dependencies if they are not already hashed themselves.
// This hash should change if:
// * t is a source file, and the contents change
// * t is a rule, and one of the following changes:
//   - Its attributes, in a meaningful way (some attributes are unordered)
//   - A hash of one of its direct dependencies
//   - The Starlark code of the producing rule
func (t *Target) getHash(w *Workspace) (uint32, error) {
	if t.hash != nil {
		return *t.hash, nil
	}

	h := fnv.New32()

	for _, dep := range t.deps {
		hash, err := dep.getHash(w)
		if err != nil {
			// TODO(scott): Log this condition
		} else {
			fmt.Fprintf(h, "%d", hash)
		}
	}

	switch t.Target.GetType() {
	case bpb.Target_RULE:
		attrList := t.Target.GetRule().GetAttribute()
		// Sort the attributes by name so they are added to the hash in a stable
		// order
		sort.Slice(attrList, func(i, j int) bool { return attrList[i].GetName() < attrList[j].GetName() })
		for _, attr := range attrList {
			fmt.Fprintf(h, "%s=%s", attr.GetName(), attrValue(attr))
		}
	case bpb.Target_SOURCE_FILE:
		lbl, err := labelFromString(t.Target.GetSourceFile().GetName())
		if err != nil {
			return 0, err
		}
		f, err := w.sourceDir.Open(lbl.filePath())
		if err != nil {
			return 0, fmt.Errorf("can't open source file %q: %w", lbl.filePath(), err)
		}
		defer f.Close()

		err = hashFile(h, f)
		if err != nil {
			// TODO(scott): After moving to go 1.17, replace this error
			// introspection with a call to Stat before Open (requires os.DirFS to
			// return an fs.StatFS). String validation is hacky, but it works for both
			// tests and prod, which return different error types so type assertion is
			// not possible here.
			if strings.Contains(err.Error(), "is a directory") {
				// Somehow a directory got passed as a label. This sometimes happens to
				// format directories onto action commandlines; in these cases, the
				// action doesn't depend on the directory's contents per se, but rather
				// the directory name. Add the name to the hash and continue.
				fmt.Fprintf(h, "%s", t.Name())
			} else {
				return 0, err
			}
		}

	case bpb.Target_GENERATED_FILE:
		// The hash of a generated file is based solely on the hash of the
		// generating rule, which was handled above by adding all deps to the hash.
		// No need to do anything more here.

	case bpb.Target_PACKAGE_GROUP:
		return 0, fmt.Errorf("PACKAGE_GROUP hashing not implemented")
	case bpb.Target_ENVIRONMENT_GROUP:
		return 0, fmt.Errorf("ENVIRONMENT_GROUP hashing not implemented")
	}

	hash := h.Sum32()
	t.hash = &hash
	return hash, nil
}

// hashFile adds the contents of a file at path `path` to the provided
// hash.Hash.
func hashFile(h hash.Hash, f io.Reader) error {
	_, err := io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("can't read file for hashing: %v", err)
	}
	return nil
}

// attrValue returns a string representation of an attribute value. This
// transformation doesn't need to be reversible, but it does need to be
// deterministic; dicts need to be sorted before serialization (although
// ordered lists should not be).
func attrValue(attr *bpb.Attribute) string {
	switch attr.GetType() {
	case bpb.Attribute_INTEGER:
		return strconv.FormatInt(int64(attr.GetIntValue()), 10)

	case bpb.Attribute_INTEGER_LIST:
		var s []string
		for _, i := range attr.GetIntListValue() {
			s = append(s, strconv.FormatInt(int64(i), 10))
		}
		// Assume that order matters here, so don't sort the strings
		return strings.Join(s, ",")

	case bpb.Attribute_BOOLEAN:
		return strconv.FormatBool(attr.GetBooleanValue())

	case bpb.Attribute_TRISTATE:
		return attr.GetTristateValue().String()

	case bpb.Attribute_STRING,
		bpb.Attribute_LABEL,
		bpb.Attribute_OUTPUT:
		return attr.GetStringValue()

	case bpb.Attribute_STRING_LIST,
		bpb.Attribute_LABEL_LIST,
		bpb.Attribute_OUTPUT_LIST,
		bpb.Attribute_DISTRIBUTION_SET:
		val := attr.GetStringListValue()
		// Assume that order matters here, so don't sort the strings
		return strings.Join(val, ",")

	case bpb.Attribute_STRING_DICT:
		val := attr.GetStringDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LABEL_DICT_UNARY:
		val := attr.GetLabelDictUnaryValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LABEL_LIST_DICT:
		val := attr.GetLabelListDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+strings.Join(entry.GetValue(), ":"))
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LABEL_KEYED_STRING_DICT:
		val := attr.GetLabelKeyedStringDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_STRING_LIST_DICT:
		val := attr.GetStringListDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+strings.Join(entry.GetValue(), ":"))
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ",")

	case bpb.Attribute_LICENSE:
		// License changes shouldn't trigger a rebuild; don't include in the hash
		return ""
	default:
		// TODO: Determine how to handle these cases
		//case bpb.Attribute_FILESET_ENTRY_LIST:
		//case bpb.Attribute_UNKNOWN:
		//case bpb.Attribute_SELECTOR_LIST:
		//case bpb.Attribute_DEPRECATED_STRING_DICT_UNARY:
		panic(fmt.Sprintf("unsupported attribute type: %v", attr.GetType()))
	}
}

// Diff returns the changed targets between this set and a baseline set.
// Ordering of the two sets is important, as targets only present in baseline
// are omitted, whereas targets only present in this set are included.
func (h TargetHashes) Diff(baseline TargetHashes) []string {
	diffs := []string{}
	for k, v := range h {
		oldVal, ok := baseline[k]
		if !ok || oldVal != v {
			diffs = append(diffs, k)
		}
	}
	return diffs
}
