package str

func ValueOrDefault(s *string, d string) string {
	if s != nil {
		return *s
	}
	return d
}

func Pointer(s string) *string {
	return &s
}
