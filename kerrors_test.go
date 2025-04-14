package kerrors

import (
	"errors"
	"regexp"
	"strings"
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

	stackRegex := regexp.MustCompile(`Stack trace\n\[\[\n\S+ \S+:\d+\n\]\]`)

	errorsErr := errors.New("test errors err")
	nestedErr := WithKind(WithMsg(errorsErr, "another message"), testErr{}, "test error message")

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
			ErrMsg: "test message 123\n[[\ntest errors err\n]]\n--\n%!(STACKTRACE)",
		},
		{
			Test:   "produces an error with an error struct kind and message",
			Opts:   []ErrorOpt{OptMsg("test message 321"), OptKind(testErr{})},
			Msg:    "test message 321",
			Kind:   testErr{},
			ErrMsg: "test message 321\n[[\ntest struct err\n]]\n--\n%!(STACKTRACE)",
		},
		{
			Test:   "produces an error with a deeply nested error",
			Opts:   []ErrorOpt{OptMsg("test message 654"), OptKind(errors.New("other error")), OptInner(nestedErr)},
			Msg:    "test message 654",
			Kind:   testErr{},
			ErrMsg: "test message 654\n[[\nother error\n]]\n--\ntest error message\n[[\ntest struct err\n]]\n--\nanother message\n--\n%!(STACKTRACE)\n--\ntest errors err",
		},
		{
			Test:   "ignores kind if not provided",
			Opts:   []ErrorOpt{OptMsg("test message 654"), OptInner(nestedErr)},
			Msg:    "test message 654",
			Kind:   testErr{},
			ErrMsg: "test message 654\n--\ntest error message\n[[\ntest struct err\n]]\n--\nanother message\n--\n%!(STACKTRACE)\n--\ntest errors err",
		},
	} {
		t.Run(tc.Test, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			err := New(tc.Opts...)
			assert.Error(err)
			msger, ok := Find[ErrorMsger](err)
			assert.True(ok)
			assert.Equal(tc.Msg, msger.ErrorMsg())
			_, ok = Find[StackStringer](err)
			assert.True(ok)
			k, ok := Find[*Error](err)
			assert.True(ok)
			assert.Equal(tc.Msg, k.Message)
			errMsg := err.Error()
			assert.Regexp(stackRegex, errMsg)
			stackstr := stackRegex.FindString(errMsg)
			assert.Contains(stackstr, "xorkevin.dev/kerrors/kerrors_test.go")
			assert.Contains(stackstr, "xorkevin.dev/kerrors.TestError")
			assert.Equal(tc.ErrMsg, stackRegex.ReplaceAllString(errMsg, "%!(STACKTRACE)"))
			if tc.Kind != nil {
				assert.ErrorIs(err, tc.Kind)
			}
			_, ok = Find[*StackTrace](err)
			assert.True(ok)
		})
	}
}

func TestStackTrace(t *testing.T) {
	t.Parallel()

	stackRegex := regexp.MustCompile(`^(?:\S+\n\t\S+:\d+\n)+$`)

	t.Run("StackString", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(0)
		stackstr := st.StackString()
		assert.Regexp(stackRegex, stackstr)
		assert.Contains(stackstr, "xorkevin.dev/kerrors/kerrors_test.go")
		assert.True(strings.HasPrefix(stackstr, "xorkevin.dev/kerrors.TestStackTrace"))
	})

	t.Run("empty stack", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(8)
		assert.Equal("", st.Error())
		assert.Equal("", st.StackString())
	})

	t.Run("As", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(0)
		stackstringer, ok := Find[StackStringer](st)
		assert.True(ok)
		assert.True(stackstringer == st)
	})
}

type (
	testBaseError struct{}

	testSingleUnwrapError struct {
		wrapped error
	}

	testAsError struct{}
)

func (e *testBaseError) Error() string {
	return "Test base error"
}

func (e *testSingleUnwrapError) Error() string {
	return "Test single unwrap error"
}

func (e *testSingleUnwrapError) Unwrap() error {
	return e.wrapped
}

func (e *testAsError) Error() string {
	return "Test as error"
}

func (e *testAsError) As(t any) bool {
	if k, ok := t.(**testBaseError); ok {
		*k = &testBaseError{}
		return true
	}
	return false
}

func TestFind(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		err, ok := Find[*testBaseError](nil)
		assert.False(ok)
		assert.True(err == nil)
	})

	t.Run("single unwrap error", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		errorsErr := &testBaseError{}
		nestedErr := &testSingleUnwrapError{wrapped: errorsErr}

		err, ok := Find[*testBaseError](nestedErr)
		assert.True(ok)
		assert.True(err == errorsErr)
	})

	t.Run("as error", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		asErr := &testAsError{}

		err, ok := Find[*testBaseError](asErr)
		assert.True(ok)
		assert.NotNil(err)
	})
}
