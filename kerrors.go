package kerrors

import (
	"encoding/json"
	"iter"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
)

type (
	// Error is an error with context
	Error struct {
		message string
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
	return e.message
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
		e.message = msg
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
	strError string
)

func (e strError) Error() string {
	return string(e)
}

func presentError(e error) error {
	if e == nil {
		return nil
	}
	if _, ok := e.(json.Marshaler); ok {
		return e
	}
	return strError(e.Error())
}

type (
	errorJSON struct {
		Message string `json:"msg"`
		Kind    error  `json:"kind,omitempty"`
		Inner   error  `json:"inner,omitempty"`
	}
)

func (e *Error) marshalObj() errorJSON {
	return errorJSON{
		Message: e.Error(),
		Kind:    presentError(e.Kind()),
		Inner:   presentError(e.Inner()),
	}
}

// MarshalJSON implements [json.Marshaler] and formats the error in json
func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.marshalObj())
}

// LogValue implements [slog.LogValuer] and returns a plain json object for
// logging
func (e *Error) LogValue() slog.Value {
	return slog.AnyValue(e.marshalObj())
}

type (
	// StackStringer returns a stack trace string
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
	b.WriteString("Stack trace (")
	if e.n > 0 {
		frameIter := runtime.CallersFrames(e.pc[:1])
		f, _ := frameIter.Next()
		e.writeStackFrame(&b, f)
	} else {
		b.WriteString("empty")
	}
	b.WriteString(")")
	return b.String()
}

// Inner returns the inner wrapped error
func (e *StackTrace) Inner() error {
	return e.wrapped
}

// Unwrap implements errors.Unwrap
func (e *StackTrace) Unwrap() error {
	return e.wrapped
}

func (e *StackTrace) PC() []uintptr {
	return e.pc[:e.n]
}

func (e *StackTrace) stackIter() iter.Seq[runtime.Frame] {
	if e.n <= 0 {
		return func(yield func(runtime.Frame) bool) {
			return
		}
	}
	return func(yield func(runtime.Frame) bool) {
		frameIter := runtime.CallersFrames(e.PC())
		for {
			f, more := frameIter.Next()
			if !yield(f) {
				return
			}
			if !more {
				return
			}
		}
	}
}

func (e *StackTrace) writeStackFrame(b *strings.Builder, f runtime.Frame) {
	b.WriteString(f.Function)
	b.WriteString(" ")
	b.WriteString(f.File)
	b.WriteString(":")
	b.WriteString(strconv.Itoa(f.Line))
}

// StackString implements [StackStringer] and formats each frame of the stack
// trace with the default format
func (e *StackTrace) StackString() string {
	var b strings.Builder
	first := true
	for f := range e.stackIter() {
		if first {
			first = false
		} else {
			b.WriteString("\n")
		}
		e.writeStackFrame(&b, f)
	}
	return b.String()
}

type (
	stackFrameJSON struct {
		Function string `json:"fn"`
		File     string `json:"file"`
		Line     int    `json:"line"`
	}

	stackTraceJSON struct {
		Message string           `json:"msg"`
		Stack   []stackFrameJSON `json:"stack,omitempty"`
		Inner   error            `json:"inner,omitempty"`
	}
)

func (e *StackTrace) marshalObj() stackTraceJSON {
	s := stackTraceJSON{
		Message: "Stack trace",
		Inner:   presentError(e.Inner()),
	}
	if e.n > 0 {
		s.Stack = make([]stackFrameJSON, 0, e.n)
	}
	for f := range e.stackIter() {
		s.Stack = append(s.Stack, stackFrameJSON{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		})
	}
	return s
}

// MarshalJSON implements [json.Marshaler] and formats each frame of the stack
// trace in json
func (e *StackTrace) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.marshalObj())
}

// LogValue implements [slog.LogValuer] and collects each frame of the stack
// trace into a log value
func (e *StackTrace) LogValue() slog.Value {
	return slog.AnyValue(e.marshalObj())
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
