package nasshp

type Verdict int

const (
	// Can't decide either way. This is useful for chaning filters.
	VerdictUnknown Verdict = iota
	// Let the request in.
	VerdictAllow
	// Block the request.
	VerdictDrop
)

func (v Verdict) MergeOnlyAcceptAllow(vv Verdict) Verdict {
	if v != VerdictAllow || vv != VerdictAllow {
		return VerdictDrop
	}
	return VerdictAllow
}
