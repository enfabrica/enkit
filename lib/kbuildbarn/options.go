package kbuildbarn

type options struct {
	Scheme       string
	PathTemplate string
	TemplateArgs []interface{}
}

type Option interface {
	apply(*options)
}

func generateOptions(base, hash, size string, inOpts ...Option) options {
	do := options{
		Scheme:       "http",
		PathTemplate: "",
		TemplateArgs: []interface{}{hash, size},
	}
	for _, o := range inOpts {
		o.apply(&do)
	}
	return do
}

// the following default values are arbitrary, based on what current works with buildbarn
const (
	DefaultFileTemplate       = "/blobs/file/%s-%s/%s"
	DefaultActionTemplate     = "/blobs/action/%s-%s"
	DefaultCommandTemplate    = "/blobs/command/%s-%s"
	DefaultDirectoryTemplate  = "/blobs/directory/%s-%s"
	DefaultByteStreamTemplate = "/blobs/%s/%s"
)

type multipleOption []Option

func (so multipleOption) apply(opts *options) {
	for _, o := range so {
		o.apply(opts)
	}
}

type schemeOption string

func (so schemeOption) apply(opts *options) {
	opts.Scheme = string(so)
}

type pathTemplateOption string

func (so pathTemplateOption) apply(opts *options) {
	opts.PathTemplate = string(so)
}

func WithActionUrlTemplate() Option {
	return pathTemplateOption(DefaultActionTemplate)
}

func WithByteStreamTemplate() Option {
	return multipleOption{pathTemplateOption(DefaultByteStreamTemplate), schemeOption("bytestream")}
}

func WithCommandUrlTemplate() Option {
	return pathTemplateOption(DefaultCommandTemplate)
}

func WithDirectoryUrlTemplate() Option {
	return pathTemplateOption(DefaultDirectoryTemplate)
}

type templateArgsOption []string

func (ta templateArgsOption) apply(opts *options) {
	s := make([]interface{}, len(ta))
	for i, v := range ta {
		s[i] = v
	}
	opts.TemplateArgs = s
}

func WithTemplateArgs(args ...string) Option {
	return templateArgsOption(args)
}

type templateAppendArgsOption []interface{}

func (ta templateAppendArgsOption) apply(opts *options) {
	opts.TemplateArgs = append(opts.TemplateArgs, ta...)
}

func WithAdditionalTemplateArgs(args ...interface{}) Option {
	return templateAppendArgsOption{args}
}

func WithFileName(s string) Option {
	return multipleOption{pathTemplateOption(DefaultFileTemplate), templateAppendArgsOption{s}}
}

type customTemplate string

func (ct customTemplate) apply(opts *options) {
	opts.PathTemplate = string(ct)
}

func WithFileTemplate(s string) Option {
	return customTemplate(s)
}
