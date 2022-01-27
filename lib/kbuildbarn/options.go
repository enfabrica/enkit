package kbuildbarn

type options struct {
	Scheme string

	FileName           string
	ByteStreamTemplate string
}

type Option interface {
	apply(*options)
}

type schemeOption string

func (so schemeOption) apply(opts *options) {
	opts.Scheme = string(so)
}

func WithScheme(s string) Option {
	return schemeOption(s)
}

type byteStreamTemplateOption string

func (so byteStreamTemplateOption) apply(opts *options) {
	opts.ByteStreamTemplate = string(so)
}

func WithByteStreamTemplate(s string) Option {
	return byteStreamTemplateOption(s)
}

type fileNameOption string

func (fno fileNameOption) apply(opts *options) {
	opts.FileName = string(fno)
}

func WithFileName(s string) Option {
	return fileNameOption(s)
}
