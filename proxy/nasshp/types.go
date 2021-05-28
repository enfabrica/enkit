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
	switch vv {
	case VerdictDrop: return VerdictDrop // no doubt, we have to drop.
	case VerdictUnknown: return v // whatever we decided before, is still valid.
	case VerdictAllow:
		if v == VerdictUnknown {
			return VerdictAllow
		}
		return v // if there was a previous Drop determination, let's stick with that. If it's not Drop, it's Allow already.
	}
	// Should never reach here
	return VerdictUnknown
}
