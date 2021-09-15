package kflags

type MapAugmenter struct {
	args map[string]string
	manglers []VarMangler
}

type MapModifier func(e *MapAugmenter)

type MapModifiers []MapModifier

func (ems MapModifiers) Apply(e *MapAugmenter) {
	for _, em := range ems {
		em(e)
	}
}

// WithMapMangler sets the list of manglers to use to lookup the parameters in the map.
func WithMapMangler(m ...VarMangler) MapModifier {
	return func(e *MapAugmenter) {
		e.manglers = m
	}
}

// By default, the map augmenter looks up the literal flag name and a camel case version of it.
//
// For example, if the 'retry-number' flag is to be filled, 'retry-number' and 'RetryNumber'
// are both looked up in the supplied map.
var DefaultVarMangler = []VarMangler{SkipNamespaceRemap(JoinRemap("")), SkipNamespaceRemap(JoinRemap("", CamelRewrite))}

// NewMapAugmenter returns an augmenter capable of looking up flags in a map.
//
// For example, supply a map like map["retry-number"] = "3", and the flag
// "retry-number" will be set to the value of 3 depending on the VarMangler
// configured.
func NewMapAugmenter(args map[string]string, mods ...MapModifier) *MapAugmenter {
	augmenter := &MapAugmenter{
		args: args,
		manglers: DefaultVarMangler,
	}

	MapModifiers(mods).Apply(augmenter)
	return augmenter
}

// VisitCommand implements the VisitCommand interface of Augmenter. In AssetAugmenter, it is a noop.
func (ma *MapAugmenter) VisitCommand(ns string, command Command) (bool, error) {
	return false, nil
}

// VisitFlag implements the VisitFlag interface of Augmenter.
func (ma *MapAugmenter) VisitFlag(reqns string, fl Flag) (bool, error) {
	tomangle := []string{fl.Name()}

	for _, mangler := range ma.manglers {
		name := mangler(tomangle...)
		if name == "" {
			continue
		}

		result, found := ma.args[name]
		if found {
			return true, fl.Set(result)
		}
	}

	return false, nil
}

// Done implements the Done interface of Augmenter.
func (ar *MapAugmenter) Done() error {
	return nil
}
