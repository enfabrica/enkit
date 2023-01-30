// Package gmap provides generic functions over maps.
package gmap

// Copy returns a deep copy of m.
func Copy[T map[K]V, K comparable, V any](m T) T {
	copy := T{}
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

// Keys returns the keys of m.
func Keys[T map[K]V, K comparable, V any](m T) []K {
	var keys []K
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// Merge returns a copy of the first arg, with each of the subsequent maps of
// the same type merged in. If map keys overlap, entry from the last map wins.
func Merge[T map[K]V, K comparable, V any](ms ...T) T {
	copy := T{}
	for _, m := range ms {
		for k, v := range m {
			copy[k] = v
		}
	}
	return copy
}
