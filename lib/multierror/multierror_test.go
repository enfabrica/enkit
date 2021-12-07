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

func TestSanityIsError(t *testing.T) {
	err := multierror.Wrap(OneErr, ThreeErr, FourErr)
	assert.True(t, errors.Is(err, OneErr))
	assert.False(t, errors.Is(err, TwoErr))
	assert.True(t, errors.Is(err, ThreeErr))
	assert.True(t, errors.Is(err, FourErr))
	realErrStrings := []string{OneErr.Error(), ThreeErr.Error(), FourErr.Error()}
	assert.Equal(t, strings.Join(realErrStrings, multierror.Seperator), err.Error())
}

func TestSanityAsError(t *testing.T) {
	tErr := &nameError{
		name: "test 1",
	}
	err := multierror.Wrap(tErr)
	var tErr1 *nameError
	assert.True(t, errors.As(err, &tErr1))
	assert.Equal(t, tErr1.Error(), tErr.Error())
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
	err := multierror.Wrap(OneErr, FourErr, SevenErr, tErr)
	newErr := multierror.Wrap(OneErr, ThreeErr, err)
	assert.True(t, errors.Is(newErr, OneErr))
	assert.False(t, errors.Is(newErr, TwoErr))
	assert.True(t, errors.Is(newErr, ThreeErr))
	assert.True(t, errors.Is(newErr, FourErr))
	assert.True(t, errors.Is(newErr, SevenErr))
	realErrStrings := []string{OneErr.Error(), ThreeErr.Error(), OneErr.Error(), FourErr.Error(), SevenErr.Error(), tErr.Error()}
	assert.Equal(t, strings.Join(realErrStrings, multierror.Seperator), newErr.Error())
	var testNameErr *nameError
	assert.True(t, errors.As(newErr, &testNameErr))
	assert.Equal(t, testNameErr.Error(), tErr.Error())
}
