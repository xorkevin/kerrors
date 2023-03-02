package kerrors

import (
	"errors"
	"fmt"
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
			ErrMsg: "test message 123\n[[\ntest errors err\n]]\n--\n%!(STACKTRACE)\n",
		},
		{
			Test:   "produces an error with an error struct kind and message",
			Opts:   []ErrorOpt{OptMsg("test message 321"), OptKind(testErr{})},
			Msg:    "test message 321",
			Kind:   testErr{},
			ErrMsg: "test message 321\n[[\ntest struct err\n]]\n--\n%!(STACKTRACE)\n",
		},
		{
			Test:   "produces an error with a deeply nested error",
			Opts:   []ErrorOpt{OptMsg("test message 654"), OptKind(errors.New("other error")), OptInner(nestedErr)},
			Msg:    "test message 654",
			Kind:   testErr{},
			ErrMsg: "test message 654\n[[\nother error\n]]\n--\ntest error message\n[[\ntest struct err\n]]\n--\nanother message\n--\n%!(STACKTRACE)\n--\ntest errors err\n",
		},
		{
			Test:   "ignores kind if not provided",
			Opts:   []ErrorOpt{OptMsg("test message 654"), OptInner(nestedErr)},
			Msg:    "test message 654",
			Kind:   testErr{},
			ErrMsg: "test message 654\n--\ntest error message\n[[\ntest struct err\n]]\n--\nanother message\n--\n%!(STACKTRACE)\n--\ntest errors err\n",
		},
	} {
		tc := tc
		t.Run(tc.Test, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			err := New(tc.Opts...)
			assert.Error(err)
			var msger ErrorMsger
			assert.ErrorAs(err, &msger)
			assert.Equal(tc.Msg, msger.ErrorMsg())
			var stackstringer StackStringer
			assert.ErrorAs(err, &stackstringer)
			var k *Error
			assert.ErrorAs(err, &k)
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
			var s *StackTrace
			assert.ErrorAs(err, &s)
		})
	}
}

func TestStackTrace(t *testing.T) {
	t.Parallel()

	stackRegex := regexp.MustCompile(`^(?:\S+\n\t\S+:\d+ \(0x[0-9a-f]+\)\n)+$`)

	t.Run("StackFormat", func(t *testing.T) {
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
		assert.Equal("", st.StackFormat(""))
	})

	t.Run("As", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(0)
		var stackstringer StackStringer
		assert.ErrorAs(st, &stackstringer)
		assert.True(stackstringer == st)
	})
}

func TestStackFrame(t *testing.T) {
	t.Parallel()

	assert := require.New(t)

	assert.Equal("someFunc file.go:127 (0x271)", fmt.Sprintf("%+v", StackFrame{
		Function: "someFunc",
		File:     "file.go",
		Line:     127,
		PC:       0x271,
	}))

	assert.Equal("someFunc file.go:127", fmt.Sprintf("%v", StackFrame{
		Function: "someFunc",
		File:     "file.go",
		Line:     127,
		PC:       0x271,
	}))

	assert.Equal("someFunc file.go:127", fmt.Sprintf("%s", StackFrame{
		Function: "someFunc",
		File:     "file.go",
		Line:     127,
		PC:       0x271,
	}))

	assert.Equal("%!z(StackFrame=someFunc file.go:127)", fmt.Sprintf("%z", StackFrame{
		Function: "someFunc",
		File:     "file.go",
		Line:     127,
		PC:       0x271,
	}))
}
