package multierror_test

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

var (
	OneErr   = errors.New("err one")
	TwoErr   = errors.New("err two")
	ThreeErr = errors.New("err three")
	FourErr  = errors.New("err four")
	FiveErr  = errors.New("err four")
	SixErr   = errors.New("err four")
	SevenErr = errors.New("err four")
)

type nameError struct {
	name string
}

func (e nameError) Error() string {
	return fmt.Sprintf("checkEven: Given number %s is not an even number", e.name)
}

type doesNotExist struct {
	Num int
}

func (exist doesNotExist) Error() string {
	return "does not exist"
}

func TestSanityIsError(t *testing.T) {
	err := multierror.Wrap(OneErr, ThreeErr, FourErr)
	assertErrIsSubErr(t, err, ThreeErr, FourErr)
	assertErrIsNotSubErr(t, err, TwoErr, FiveErr, SevenErr)
	assertErrContainsAllSubStrings(t, err, ThreeErr, FourErr, OneErr)
}

func TestSanityAsError(t *testing.T) {
	tErr := &nameError{
		name: "test 1",
	}
	err := multierror.Wrap(tErr)
	var tErr1 *nameError
	assert.True(t, errors.As(err, &tErr1))
	assert.Equal(t, tErr1.Error(), tErr.Error())
	var doesNotExistErr *doesNotExist
	assert.False(t, errors.As(err, &doesNotExistErr))
}

func TestManyNestedMultiErr(t *testing.T) {
	tErr := &nameError{name: "nested many"}
	m := multierror.Wrap(OneErr, tErr)
	for i := 0; i < 1000; i++ {
		m = multierror.Wrap(ThreeErr, FourErr, m)
	}
	var testNameErr *nameError
	assert.True(t, errors.As(m, &testNameErr))
	assert.Equal(t, testNameErr.Error(), tErr.Error())
	assert.True(t, errors.Is(m, OneErr))
}

func TestSingleNestedMultiErr(t *testing.T) {
	tErr := &nameError{"TestSingleNestedMultiErr"}
	subErr := multierror.Wrap(OneErr, FourErr, SevenErr, tErr)
	err := multierror.Wrap(OneErr, ThreeErr, subErr)
	assertErrIsSubErr(t, err, OneErr, ThreeErr, FourErr, SevenErr)
	assertErrIsNotSubErr(t, err, TwoErr, SixErr)
	assertErrContainsAllSubStrings(t, err, OneErr, ThreeErr, OneErr, FourErr, SevenErr, tErr)
	var testNameErr *nameError
	assert.True(t, errors.As(err, &testNameErr))
	assert.Equal(t, testNameErr.Error(), tErr.Error())
}

func assertErrContainsAllSubStrings(t *testing.T, primary error, rest ...error) {
	for _, err := range rest {
		assert.True(t, strings.Contains(primary.Error(), err.Error()))
	}
}

func assertErrIsSubErr(t *testing.T, primary error, rest ...error) {
	for _, err := range rest {
		assert.True(t, errors.Is(primary, err))
	}
}

func assertErrIsNotSubErr(t *testing.T, primary error, rest ...error) {
	for _, err := range rest {
		assert.False(t, errors.Is(primary, err))
	}
}
