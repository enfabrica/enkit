package machinist

type SharedMachinistFlags struct {
	Port uint32
}

type SharedFlagModifier func(smf *SharedMachinistFlags) error

func WithPort(p uint32) SharedFlagModifier {
	return func(smf *SharedMachinistFlags) error {
		smf.Port = p
		return nil
	}
}