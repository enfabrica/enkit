package slice

// ToSet returns a map that acts as a set from the contents of slice s.
func ToSet[T comparable](s []T) map[T]struct{} {
	m := map[T]struct{}{}
	for _, elem := range s {
		m[elem] = struct{}{}
	}
	return m
}
