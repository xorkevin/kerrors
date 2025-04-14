package kerrors

import (
	"errors"
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

	// ErrorOpt is an error options function used by [New]
	ErrorOpt = func(e *Error)

	// ErrorWriter writes errors to an [io.Writer]
	ErrorWriter interface {
		WriteError(b io.Writer)
	}

	// ErrorMsger returns the error message
	ErrorMsger interface {
		ErrorMsg() string
	}
)

// New creates a new [*Error]
func New(opts ...ErrorOpt) error {
	e := &Error{
		skip: 0,
	}
	for _, i := range opts {
		i(e)
	}
	e.wrapped[1] = addStackTrace(e.Inner(), 1+e.skip)
	return e
}

// WriteError implements [ErrorWriter]
func (e *Error) WriteError(b io.Writer) {
	io.WriteString(b, e.Message)
	if kind := e.Kind(); kind != nil {
		io.WriteString(b, "\n[[\n")
		if k, ok := kind.(ErrorWriter); ok {
			k.WriteError(b)
		} else {
			io.WriteString(b, kind.Error())
		}
		io.WriteString(b, "\n]]")
	}
	if inner := e.Inner(); inner != nil {
		io.WriteString(b, "\n--\n")
		if k, ok := inner.(ErrorWriter); ok {
			k.WriteError(b)
		} else {
			io.WriteString(b, inner.Error())
		}
	}
}

// Error implements error and recursively prints wrapped errors
func (e *Error) Error() string {
	var b strings.Builder
	e.WriteError(&b)
	return b.String()
}

// ErrorMsg returns the error message
func (e *Error) ErrorMsg() string {
	return e.Message
}

// Unwrap implements [errors.Unwrap]
func (e *Error) Unwrap() []error {
	start := 0
	end := 2
	if e.wrapped[0] == nil {
		start = 1
	}
	if e.wrapped[1] == nil {
		end = 1
	}
	return e.wrapped[start:end]
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
		n  int
		pc [128]uintptr
	}
)

// NewStackTrace creates a new [*StackTrace]
func NewStackTrace(skip int) *StackTrace {
	e := &StackTrace{}
	e.n = runtime.Callers(2+skip, e.pc[:])
	return e
}

// WriteError implements [ErrorWriter] and writes the stack trace
func (e *StackTrace) WriteError(b io.Writer) {
	if e.n <= 0 {
		return
	}
	frameIter := runtime.CallersFrames(e.pc[:1])
	f, _ := frameIter.Next()
	io.WriteString(b, f.Function)
	io.WriteString(b, " ")
	io.WriteString(b, f.File)
	io.WriteString(b, ":")
	io.WriteString(b, strconv.Itoa(f.Line))
}

// Error implements error and prints the stack trace
func (e *StackTrace) Error() string {
	var b strings.Builder
	e.WriteError(&b)
	return b.String()
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

func addStackTrace(err error, skip int) error {
	var e *StackTrace
	if err != nil && errors.As(err, &e) {
		return err
	}
	// construct an error to avoid infinite recursive loop
	return &Error{
		Message: "Stack trace",
		wrapped: [2]error{NewStackTrace(1 + skip), err},
		skip:    0,
	}
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
	errorAser interface {
		As(any) bool
	}

	errorUnwrapper interface {
		Unwrap() []error
	}

	errorSingleUnwrapper interface {
		Unwrap() error
	}
)

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
