package metrics

import (
	"fmt"
	"strings"
)

// OneofLabel returns a string that can be used as a metric label for a oneof
// type.
//
// In Go, Oneofs usually have a type like ParentMessage_FieldName or
// ParentMessage_NestedType_FieldName; this function will return just FieldName.
func OneofLabel(t any) string {
	s := fmt.Sprintf("%T", t)
	pieces := strings.Split(s, "_")
	return pieces[len(pieces)-1]
}
