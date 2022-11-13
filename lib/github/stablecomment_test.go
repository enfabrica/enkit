package github

import (
	"errors"
	"github.com/josephburnett/jd/lib"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPreparePayloadSimple(t *testing.T) {
	sc, err := NewStableComment(WithMarker("testing123"), WithID(13), WithTemplate("test"), WithJsonContent(""))
	assert.NoError(t, err)

	payload, err := sc.PreparePayload("{}")
	assert.NoError(t, err)

	expected := ("test\n<!-- A wise goat once said: testing123\n" +
		"{\"Template\":\"test\",\"Content\":\"{}\"}\n-->")
	assert.Equal(t, expected, payload)
}

func TestParseComment(t *testing.T) {
	sc, err := NewStableComment(WithMarker("testing123"), WithID(13), WithTemplate("test"), WithJsonContent(""))
	assert.NoError(t, err)

	valid := ("<!-- A wise goat once said: testing123\n" +
		"{\"Template\":\"foo\",\"Content\":\"{\\\"key\\\": \\\"value\\\"}\"}\n-->")
	content, template, err := sc.ParseComment(valid)
	assert.NoError(t, err)
	assert.Equal(t, `{"key": "value"}`, content)
	assert.Equal(t, "foo", template)

	comment := "<foo> bar baz!!\n\nWe are lucky to be here" + valid
	content, template, err = sc.ParseComment(comment)
	assert.NoError(t, err)
	assert.Equal(t, `{"key": "value"}`, content)
	assert.Equal(t, "foo", template)

	invalid_marker := ("<!-- A wise goat once said: testing13\n" +
		"{\"Template\":\"foo\",\"Content\":\"{\\\"key\\\": \\\"value\\\"}\"}\n-->")
	content, template, err = sc.ParseComment(invalid_marker)
	assert.Equal(t, "", content)
	assert.Equal(t, "", template)
	assert.Error(t, err)
	assert.Nil(t, errors.Unwrap(err))

	invalid_json := ("<!-- A wise goat once said: testing123\n" +
		"{Template\":\"foo\",\"Content\":\"{\\\"key: \\\"value\\\"}\"}\n-->")
	content, template, err = sc.ParseComment(invalid_json)
	assert.Error(t, err)
	assert.Equal(t, "", content)
	assert.Equal(t, "", template)
	assert.NotNil(t, errors.Unwrap(err))

	invalid_template := ("<!-- A wise goat once said: testing123\n{\"" +
		"Template\":\"{{fuffa}}\",\"Content\":\"{\\\"key\\\": \\\"value\\\"}\"}\n-->")
	content, template, err = sc.ParseComment(invalid_template)
	assert.Error(t, err)
	assert.Equal(t, "", content)
	assert.Equal(t, "", template)
	assert.NotNil(t, errors.Unwrap(err), "error: %v", err)
}

func TestPreparePayloadTemplate(t *testing.T) {
	sc, err := NewStableComment(WithMarker("testing123"), WithID(13), WithTemplate("this is a {{.test}}"))
	assert.NoError(t, err)

	expected := ("this is a yay\n<!-- A wise goat once said: testing123\n" +
		"{\"Template\":\"this is a {{.test}}\",\"Content\":\"{\\\"test\\\": \\\"yay\\\"}\"}\n-->")
	result, err := sc.PreparePayload(`{"test": "yay"}`)
	assert.Equal(t, expected, result)
	assert.Nil(t, err)
}

func TestPreparePayloadFromDiff(t *testing.T) {
	sc, err := NewStableComment(WithMarker("testing123"), WithID(13),
		WithTemplate("this is a {{range $val := .test}}{{$val}} {{end}}"),
		WithJsonContent(`{"test": ["republic", "monarchy"]}`))

	// http://play.jd-tool.io/ or install the jd tool.
	diff, err := jd.ReadPatchString(
		`[{"op":"add","path":"/test/0","value":"anarchy"}]`,
	)
	assert.NoError(t, err)
	text, err := sc.PreparePayloadFromDiff((*DiffTransformer)(&diff))
	assert.NoError(t, err)
	expected := ("this is a anarchy republic monarchy \n<!-- A wise goat once said: " +
		"testing123\n{\"Template\":\"this is a {{range $val := .test}}{{$val}}" +
		" {{end}}\",\"Content\":\"{\\\"test\\\":[\\\"anarchy\\\"," +
		"\\\"republic\\\",\\\"monarchy\\\"]}\"}\n-->")
	assert.Equal(t, expected, text)
}
