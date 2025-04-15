package kerrors

import (
	"encoding/json"
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

func searchMapKey(data any, key string) map[string]any {
	switch v := data.(type) {
	case map[string]any:
		if _, ok := v[key]; ok {
			return v
		}
		for _, val := range v {
			if result := searchMapKey(val, key); result != nil {
				return result
			}
		}
	case []any:
		for _, val := range v {
			if result := searchMapKey(val, key); result != nil {
				return result
			}
		}
	}
	return nil
}

func TestError(t *testing.T) {
	t.Parallel()

	errorsErr := errors.New("test errors err")
	nestedErr := WithKind(WithMsg(errorsErr, "another message"), testErr{}, "test error message")

	for _, tc := range []struct {
		Test   string
		Opts   []ErrorOpt
		Kind   error
		ErrMsg any
	}{
		{
			Test: "produces an error with an errors kind and message",
			Opts: []ErrorOpt{OptMsg("test message 123"), OptKind(errorsErr)},
			Kind: errorsErr,
			ErrMsg: map[string]any{
				"msg":  "test message 123",
				"kind": "test errors err",
				"inner": map[string]any{
					"msg": "Stack trace",
				},
			},
		},
		{
			Test: "produces an error with an error struct kind and message",
			Opts: []ErrorOpt{OptMsg("test message 321"), OptKind(testErr{})},
			Kind: testErr{},
			ErrMsg: map[string]any{
				"msg":  "test message 321",
				"kind": "test struct err",
				"inner": map[string]any{
					"msg": "Stack trace",
				},
			},
		},
		{
			Test: "produces an error with a deeply nested error",
			Opts: []ErrorOpt{OptMsg("test message 654"), OptKind(errors.New("other error")), OptInner(nestedErr)},
			Kind: testErr{},
			ErrMsg: map[string]any{
				"msg":  "test message 654",
				"kind": "other error",
				"inner": map[string]any{
					"msg":  "test error message",
					"kind": "test struct err",
					"inner": map[string]any{
						"msg": "another message",
						"inner": map[string]any{
							"msg":   "Stack trace",
							"inner": "test errors err",
						},
					},
				},
			},
		},
		{
			Test: "ignores kind if not provided",
			Opts: []ErrorOpt{OptMsg("test message 654"), OptInner(nestedErr)},
			Kind: testErr{},
			ErrMsg: map[string]any{
				"msg": "test message 654",
				"inner": map[string]any{
					"msg":  "test error message",
					"kind": "test struct err",
					"inner": map[string]any{
						"msg": "another message",
						"inner": map[string]any{
							"msg":   "Stack trace",
							"inner": "test errors err",
						},
					},
				},
			},
		},
		{
			Test: "finds error kinds through stack traces",
			Opts: []ErrorOpt{OptMsg("test message 654"), OptInner(nestedErr)},
			Kind: errorsErr,
			ErrMsg: map[string]any{
				"msg": "test message 654",
				"inner": map[string]any{
					"msg":  "test error message",
					"kind": "test struct err",
					"inner": map[string]any{
						"msg": "another message",
						"inner": map[string]any{
							"msg":   "Stack trace",
							"inner": "test errors err",
						},
					},
				},
			},
		},
	} {
		t.Run(tc.Test, func(t *testing.T) {
			t.Parallel()

			assert := require.New(t)

			err := New(tc.Opts...)
			assert.Error(err)
			b, jerr := json.Marshal(err)
			assert.NoError(jerr)
			var errMsg map[string]any
			assert.NoError(json.Unmarshal(b, &errMsg))
			stackTrace := searchMapKey(errMsg, "stack")
			assert.NotNil(stackTrace)
			stack, ok := stackTrace["stack"].([]any)
			assert.True(ok)
			assert.NotNil(stack)
			assert.Contains(stack[0].(map[string]any)["file"], "xorkevin.dev/kerrors/kerrors_test.go")
			assert.Contains(stack[0].(map[string]any)["fn"], "xorkevin.dev/kerrors.TestError")
			delete(stackTrace, "stack")
			assert.Equal(tc.ErrMsg, errMsg)
			assert.ErrorIs(err, tc.Kind)
			kerr, ok := Find[*Error](err)
			assert.True(ok)
			assert.True(kerr.Inner() == kerr.wrapped[1])
			assert.True(kerr.Kind() == kerr.wrapped[0])
			assert.True(kerr.Error() == kerr.message)
			_, ok = Find[StackStringer](err)
			assert.True(ok)
			_, ok = Find[*StackTrace](err)
			assert.True(ok)
		})
	}
}

func TestStackTrace(t *testing.T) {
	t.Parallel()

	stackRegex := regexp.MustCompile(`^(?:\S+ \S+:\d+)(?:\n\S+ \S+:\d+)*$`)

	t.Run("StackString", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(nil, 0)
		stackstr := st.StackString()
		assert.Regexp(stackRegex, stackstr)
		assert.Contains(stackstr, "xorkevin.dev/kerrors/kerrors_test.go")
		assert.True(strings.HasPrefix(stackstr, "xorkevin.dev/kerrors.TestStackTrace"))
	})

	t.Run("empty stack", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(nil, 8)
		assert.Equal("Stack trace (empty)", st.Error())
		assert.Equal("", st.StackString())
	})

	t.Run("As", func(t *testing.T) {
		t.Parallel()

		assert := require.New(t)

		st := NewStackTrace(nil, 0)
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
