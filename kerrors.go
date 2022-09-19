package kerrors

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
)

type (
	// Error is an error with context
	Error struct {
		Message string
		Kind    error
		Inner   error
		skip    int
	}

	// ErrorOpt is an error options function used by [New]
	ErrorOpt = func(e *Error)

	// ErrorWriter writes errors to an [io.Writer]
	ErrorWriter interface {
		WriteError(b io.Writer)
	}
)

// New creates a new [*Error]
func New(opts ...ErrorOpt) error {
	e := &Error{
		// assume that the caller wants the line at which New is called
		skip: 1,
	}
	for _, i := range opts {
		i(e)
	}
	e.Inner = addStackTrace(e.Inner, 1+e.skip)
	return e
}

// WriteError implements [ErrorWriter]
func (e *Error) WriteError(b io.Writer) {
	io.WriteString(b, e.Message)
	if e.Kind != nil {
		io.WriteString(b, " [")
		if k, ok := e.Kind.(ErrorWriter); ok {
			k.WriteError(b)
		} else {
			io.WriteString(b, e.Kind.Error())
		}
		io.WriteString(b, "]")
	}
	if e.Inner != nil {
		io.WriteString(b, ": ")
		if k, ok := e.Inner.(ErrorWriter); ok {
			k.WriteError(b)
		} else {
			io.WriteString(b, e.Inner.Error())
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

// OptInner returns an [ErrorOpt] that sets [Error] Inner
func OptInner(inner error) ErrorOpt {
	return func(e *Error) {
		e.Inner = inner
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

	// StackStringerSetter sets a StackStringer
	StackStringerSetter interface {
		SetStackStringer(s StackStringer)
	}

	// StackTrace is an error stack trace
	StackTrace struct {
		n  int
		pc [128]uintptr
	}

	// StackFrame is a stack trace frame
	StackFrame struct {
		Function string
		File     string
		Line     int
		PC       uintptr
	}
)

// NewStackTrace creates a new [*StackTrace]
func NewStackTrace(skip int) *StackTrace {
	e := &StackTrace{}
	e.n = runtime.Callers(1+skip, e.pc[:])
	return e
}

// WriteError implements [ErrorWriter] and writes the stack trace
func (e *StackTrace) WriteError(b io.Writer) {
	if e.n <= 0 {
		return
	}
	frameIter := runtime.CallersFrames(e.pc[:1])
	f, _ := frameIter.Next()
	fmt.Fprint(b, runtimeFrameToFrame(f))
}

// Error implements error and prints the stack trace
func (e *StackTrace) Error() string {
	b := strings.Builder{}
	e.WriteError(&b)
	return b.String()
}

// StackFormat formats each frame of the stack trace with the format specifier
func (e *StackTrace) StackFormat(format string) string {
	if e.n <= 0 {
		return ""
	}
	b := strings.Builder{}
	frameIter := runtime.CallersFrames(e.pc[:e.n])
	for {
		f, more := frameIter.Next()
		fmt.Fprintf(&b, format, runtimeFrameToFrame(f))
		if !more {
			break
		}
	}
	return b.String()
}

// StackString implements [StackStringer] and formats each frame of the stack
// trace with the default format
func (e *StackTrace) StackString() string {
	return e.StackFormat("%[1]f\n\t%[1]e:%[1]l (0x%[1]c)\n")
}

// As implements [errors.As]
func (e *StackTrace) As(target interface{}) bool {
	if s, ok := target.(StackStringerSetter); ok {
		s.SetStackStringer(e)
		return true
	}
	return false
}

// Format implements [fmt.Formatter]
//
//   - %f   function name
//   - %e   file path
//   - %l   file line number
//   - %c   program counter in hex
//   - %s   equivalent to error string "%f %e:%l"
//   - %v   equivalent to error string "%f %e:%l"
//   - %+v  equivalent to stack string "%f %e:%l (0x%c)"
func (f StackFrame) Format(s fmt.State, verb rune) {
	switch verb {
	case 'f':
		io.WriteString(s, f.Function)
	case 'e':
		io.WriteString(s, f.File)
	case 'l':
		io.WriteString(s, strconv.Itoa(f.Line))
	case 'c':
		io.WriteString(s, strconv.FormatUint(uint64(f.PC), 16))
	case 's':
		fmt.Fprintf(s, "%s %s:%s", f.Function, f.File, strconv.Itoa(f.Line))
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%s %s:%s (0x%s)", f.Function, f.File, strconv.Itoa(f.Line), strconv.FormatUint(uint64(f.PC), 16))
		} else {
			fmt.Fprintf(s, "%s %s:%s", f.Function, f.File, strconv.Itoa(f.Line))
		}
	default:
		fmt.Fprintf(s, "%%!%c(StackFrame=%v)", verb, f)
	}
}

func runtimeFrameToFrame(f runtime.Frame) StackFrame {
	return StackFrame{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
		PC:       f.PC,
	}
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
	return New(OptMsg(msg), OptInner(err), OptSkip(1))
}

// WithKind returns an error wrapped by an [*Error] with a Kind and Message
func WithKind(err error, kind error, msg string) error {
	return New(OptMsg(msg), OptKind(kind), OptInner(err), OptSkip(1))
}
