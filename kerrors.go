package kerrors

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
)

type (
	// Error is an error with context
	Error struct {
		Message string
		Kind    error
		Inner   error
	}

	// ErrorOpt is an error options function used by [New]
	ErrorOpt = func(e *Error)

	// ErrorWriter writes errors to an [io.StringWriter]
	ErrorWriter interface {
		WriteError(b io.StringWriter)
	}
)

// New creates a new [*Error]
func New(opts ...ErrorOpt) error {
	e := &Error{}
	for _, i := range opts {
		i(e)
	}
	e.Inner = addStackTrace(e.Inner, 2)
	return e
}

// WriteError implements [ErrorWriter]
func (e *Error) WriteError(b io.StringWriter) {
	b.WriteString(e.Message)
	if e.Kind != nil {
		b.WriteString(" [")
		if k, ok := e.Kind.(ErrorWriter); ok {
			k.WriteError(b)
		} else {
			b.WriteString(e.Kind.Error())
		}
		b.WriteString("]")
	}
	if e.Inner != nil {
		b.WriteString(": ")
		if k, ok := e.Inner.(ErrorWriter); ok {
			k.WriteError(b)
		} else {
			b.WriteString(e.Inner.Error())
		}
	}
}

// Error implements error and recursively prints wrapped errors
func (e *Error) Error() string {
	b := strings.Builder{}
	e.WriteError(&b)
	return b.String()
}

// Unwrap implements [errors.Unwrap]
func (e *Error) Unwrap() error {
	return e.Inner
}

// Is implements [errors.Is]
func (e *Error) Is(target error) bool {
	if e.Kind == nil {
		return false
	}
	return errors.Is(e.Kind, target)
}

// As implements [errors.As]
func (e *Error) As(target interface{}) bool {
	if e.Kind == nil {
		return false
	}
	return errors.As(e.Kind, target)
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
		e.Kind = kind
	}
}

// OptInner returns an [ErrorOpt] that sets [Error.Inner]
func OptInner(inner error) ErrorOpt {
	return func(e *Error) {
		e.Inner = inner
	}
}

type (
	// StackTrace is an error stack trace
	StackTrace struct {
		n  int
		pc [128]uintptr
	}
)

// NewStackTrace creates a new [*StackTrace]
func NewStackTrace(skip int) *StackTrace {
	e := &StackTrace{}
	e.n = runtime.Callers(1+skip, e.pc[:])
	return e
}

// WriteError implements [ErrorWriter] and writes the stack trace
func (e *StackTrace) WriteError(b io.StringWriter) {
	b.WriteString("\n")
	if e.n <= 0 {
		return
	}
	frameIter := runtime.CallersFrames(e.pc[:e.n])
	for {
		frame, more := frameIter.Next()
		b.WriteString(fmt.Sprintf("%s\n\t%s:%d (0x%x)\n", frame.Function, frame.File, frame.Line, frame.PC))
		if !more {
			break
		}
	}
}

// Error implements error and prints the stack trace
func (e *StackTrace) Error() string {
	b := strings.Builder{}
	e.WriteError(&b)
	return b.String()
}

func addStackTrace(err error, skip int) error {
	var e *StackTrace
	if err != nil && errors.As(err, &e) {
		return err
	}
	return &Error{
		Message: "Stack trace",
		Kind:    NewStackTrace(1 + skip),
		Inner:   err,
	}
}

// WithMsg returns an error wrapped by an [*Error] with a Message
func WithMsg(err error, msg string) error {
	return New(OptMsg(msg), OptInner(err))
}

// WithKind returns an error wrapped by an [*Error] with a Kind and Message
func WithKind(err error, kind error, msg string) error {
	return New(OptMsg(msg), OptKind(kind), OptInner(err))
}
