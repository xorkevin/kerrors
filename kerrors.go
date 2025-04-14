package kerrors

import (
	"io"
	"runtime"
	"strconv"
	"strings"
)

type (
	// Error is an error with context
	Error struct {
		Message string
		wrapped [2]error
		skip    int
	}

	// ErrorOpt is an options function used by [New]
	ErrorOpt = func(e *Error)
)

// New creates a new [*Error]
func New(opts ...ErrorOpt) error {
	e := &Error{
		skip: 0,
	}
	for _, i := range opts {
		i(e)
	}
	e.wrapped[1] = AddStackTrace(e.wrapped[1], 1+e.skip)
	return e
}

// Error implements error
func (e *Error) Error() string {
	return e.Message
}

// Unwrap implements [errors.Unwrap]
func (e *Error) Unwrap() []error {
	start := 0
	if e.wrapped[0] == nil {
		start = 1
	}
	// wrapped 1 will always be populated because of stack trace
	return e.wrapped[start:2]
}

// Kind returns the error kind
func (e *Error) Kind() error {
	return e.wrapped[0]
}

// Inner returns the inner wrapped error
func (e *Error) Inner() error {
	return e.wrapped[1]
}

// OptMsg returns an [ErrorOpt] that sets [Error] Message
func OptMsg(msg string) ErrorOpt {
	return func(e *Error) {
		e.Message = msg
	}
}

// OptKind returns an [ErrorOpt] that sets [Error] Kind
func OptKind(kind error) ErrorOpt {
	return func(e *Error) {
		e.wrapped[0] = kind
	}
}

// OptInner returns an [ErrorOpt] that sets [Error] Inner
func OptInner(inner error) ErrorOpt {
	return func(e *Error) {
		e.wrapped[1] = inner
	}
}

// OptSkip returns an [ErrorOpt] that increments [Error] skip by a number of
// frames for stack trace
func OptSkip(skip int) ErrorOpt {
	return func(e *Error) {
		e.skip += skip
	}
}

type (
	// StackStringer returns a stacktrace string
	StackStringer interface {
		StackString() string
	}

	// StackTrace is an error stack trace
	StackTrace struct {
		wrapped error
		n       int
		pc      [128]uintptr
	}
)

// NewStackTrace creates a new [*StackTrace]
func NewStackTrace(err error, skip int) *StackTrace {
	e := &StackTrace{
		wrapped: err,
	}
	e.n = runtime.Callers(2+skip, e.pc[:])
	return e
}

// Error implements error and prints the stack trace
func (e *StackTrace) Error() string {
	var b strings.Builder
	io.WriteString(&b, "Stack trace (")
	if e.n > 0 {
		frameIter := runtime.CallersFrames(e.pc[:1])
		f, _ := frameIter.Next()
		io.WriteString(&b, f.Function)
		io.WriteString(&b, " ")
		io.WriteString(&b, f.File)
		io.WriteString(&b, ":")
		io.WriteString(&b, strconv.Itoa(f.Line))
	} else {
		io.WriteString(&b, "empty")
	}
	io.WriteString(&b, ")")
	return b.String()
}

// Unwrap implements errors.Unwrap
func (e *StackTrace) Unwrap() error {
	return e.wrapped
}

func (e *StackTrace) PC() []uintptr {
	return e.pc[:e.n]
}

// StackString implements [StackStringer] and formats each frame of the stack
// trace with the default format
func (e *StackTrace) StackString() string {
	if e.n <= 0 {
		return ""
	}
	var b strings.Builder
	frameIter := runtime.CallersFrames(e.PC())
	for {
		f, more := frameIter.Next()
		b.WriteString(f.Function)
		b.WriteString("\n\t")
		b.WriteString(f.File)
		b.WriteString(":")
		b.WriteString(strconv.Itoa(f.Line))
		b.WriteByte('\n')
		if !more {
			break
		}
	}
	return b.String()
}

// AddStackTrace adds a [*StackTrace] if one is not already present in the
// error chain
func AddStackTrace(err error, skip int) error {
	if _, ok := Find[*StackTrace](err); ok {
		return err
	}
	return NewStackTrace(err, 1+skip)
}

// WithMsg returns an error wrapped by an [*Error] with a Message
func WithMsg(err error, msg string) error {
	return New(OptMsg(msg), OptInner(err), OptSkip(1))
}

// WithKind returns an error wrapped by an [*Error] with a Kind and Message
func WithKind(err error, kind error, msg string) error {
	return New(OptMsg(msg), OptKind(kind), OptInner(err), OptSkip(1))
}

type (
	errorUnwrapper interface {
		Unwrap() []error
	}

	errorSingleUnwrapper interface {
		Unwrap() error
	}

	errorAser interface {
		As(any) bool
	}
)

// Find finds an error of type T in the error chain using [errors.As] rules
func Find[T any](err error) (T, bool) {
	if err == nil {
		var t T
		return t, false
	}

	if t, ok := err.(T); ok {
		return t, true
	}

	switch k := err.(type) {
	case errorAser:
		{
			var t T
			if k.As(&t) {
				return t, true
			}
		}
	case errorUnwrapper:
		for _, e := range k.Unwrap() {
			if t, ok := Find[T](e); ok {
				return t, true
			}
		}
	case errorSingleUnwrapper:
		return Find[T](k.Unwrap())
	}
	var t T
	return t, false
}

// WriteError writes errors recursively through the error chain
func WriteError(b io.Writer, target error) error {
	if target == nil {
		return nil
	}

	if _, err := io.WriteString(b, target.Error()); err != nil {
		return err
	}

	var wrapped []error
	switch k := target.(type) {
	case errorUnwrapper:
		wrapped = k.Unwrap()
	case errorSingleUnwrapper:
		if e := k.Unwrap(); e != nil {
			wrapped = []error{e}
		}
	}

	noWrappedError := true
	for _, i := range wrapped {
		if i != nil {
			noWrappedError = false
			break
		}
	}
	if noWrappedError {
		return nil
	}

	if _, err := io.WriteString(b, "\n--"); err != nil {
		return err
	}
	for _, i := range wrapped {
		if i == nil {
			continue
		}
		if _, err := io.WriteString(b, "\n[[\n"); err != nil {
			return err
		}
		if err := WriteError(b, i); err != nil {
			return err
		}
		if _, err := io.WriteString(b, "\n]]"); err != nil {
			return err
		}
	}
	return nil
}
