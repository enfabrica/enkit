package machinist
//
//import (
//	"github.com/enfabrica/enkit/lib/client"
//	"github.com/enfabrica/enkit/lib/kflags"
//	"github.com/enfabrica/enkit/lib/logger"
//	"github.com/enfabrica/enkit/lib/retry"
//)
//
//type Flags struct {
//	*client.ServerFlags
//	*retry.Flags
//}
//
//func DefaultFlags() *Flags {
//	flags := &Flags{
//		ServerFlags: client.DefaultServerFlags("controller", "Controller", ""),
//		Flags:       retry.DefaultFlags(),
//	}
//	flags.AtMost = 0 // By default, retry an infinite amount of times.
//	return flags
//}
//
//func (fl *Flags) Register(fs kflags.FlagSet, prefix string) *Flags {
//	fl.ServerFlags.Register(fs, prefix)
//	fl.Flags.Register(fs, prefix)
//	return fl
//}
//
//type Modifier func(*Machinist) error
//
//type Modifiers []Modifier
//
//func (ms Modifiers) Apply(o *Machinist) error {
//	for _, mod := range ms {
//		if err := mod(o); err != nil {
//			return err
//		}
//	}
//	return nil
//}
//
//func FromFlags(flags *Flags) Modifier {
//	return func(m *Machinist) error {
//		m.rmods = append(m.rmods, retry.FromFlags(flags.Flags))
//		m.server = flags.ServerFlags
//		return nil
//	}
//}
//
//func WithServerFlags(sf *client.ServerFlags) Modifier {
//	return func(m *Machinist) error {
//		m.server = sf
//		return nil
//	}
//}
//
//func WithLogger(log logger.Logger) Modifier {
//	return func(m *Machinist) error {
//		m.log = log
//		return nil
//	}
//}
//
//func WithServerOptions(mods ...client.GwcOrGrpcOptions) Modifier {
//	return func(m *Machinist) error {
//		m.smods = append(m.smods, mods...)
//		return nil
//	}
//}
//func WithRetryOptions(mods ...retry.Modifier) Modifier {
//	return func(m *Machinist) error {
//		m.rmods = append(m.rmods, mods...)
//		return nil
//	}
//}
