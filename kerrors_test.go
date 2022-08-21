package kerrors

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	testErr struct{}
)

func (e testErr) Error() string {
	return "test struct err"
}

func TestError(t *testing.T) {
	t.Parallel()

	errorsErr := errors.New("test errors err")
	nestedErr := WithKind(WithMsg(errorsErr, "another message"), testErr{}, "test error message")

	stackRegex := regexp.MustCompile(`Stack trace \[\n(?:\S*\n\t\S+:\d+ \(0x[0-9a-f]+\)\n)+\]`)

	for _, tc := range []struct {
		Test   string
		Opts   []ErrorOpt
		Msg    string
		Kind   error
		ErrMsg string
	}{
		{
			Test:   "produces an error with an errors kind and message",
			Opts:   []ErrorOpt{OptMsg("test message 123"), OptKind(errorsErr)},
			Msg:    "test message 123",
			Kind:   errorsErr,
			ErrMsg: "test message 123 [test errors err]: %!(STACKTRACE)",
		},
		{
			Test:   "produces an error with an error struct kind and message",
			Opts:   []ErrorOpt{OptMsg("test message 321"), OptKind(testErr{})},
			Msg:    "test message 321",
			Kind:   testErr{},
			ErrMsg: "test message 321 [test struct err]: %!(STACKTRACE)",
		},
		{
			Test:   "produces an error with a deeply nested error",
			Opts:   []ErrorOpt{OptMsg("test message 654"), OptKind(errors.New("other error")), OptInner(nestedErr)},
			Msg:    "test message 654",
			Kind:   testErr{},
			ErrMsg: "test message 654 [other error]: test error message [test struct err]: another message: %!(STACKTRACE): test errors err",
		},
		{
			Test:   "ignores kind if not provided",
			Opts:   []ErrorOpt{OptMsg("test message 654"), OptInner(nestedErr)},
			Msg:    "test message 654",
			Kind:   testErr{},
			ErrMsg: "test message 654: test error message [test struct err]: another message: %!(STACKTRACE): test errors err",
		},
	} {
		tc := tc
		t.Run(tc.Test, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			err := New(tc.Opts...)
			assert.Error(err)
			var k *Error
			assert.ErrorAs(err, &k)
			assert.Equal(tc.Msg, k.Message)
			errMsg := err.Error()
			assert.Regexp(stackRegex, errMsg)
			assert.Equal(tc.ErrMsg, stackRegex.ReplaceAllString(errMsg, "%!(STACKTRACE)"))
			if tc.Kind != nil {
				assert.ErrorIs(err, tc.Kind)
			}
			var s *StackTrace
			assert.ErrorAs(err, &s)
		})
	}
}

func TestStackTrace(t *testing.T) {
	t.Parallel()

	assert := require.New(t)
	assert.Equal("\n", NewStackTrace(8).Error())
}
