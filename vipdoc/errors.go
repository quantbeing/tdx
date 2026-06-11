package vipdoc

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidInput      = errors.New("vipdoc: invalid input")
	ErrInvalidRoot       = errors.New("vipdoc: invalid root")
	ErrTruncatedFile     = errors.New("vipdoc: truncated file")
	ErrUnsupportedMarket = errors.New("vipdoc: unsupported market")
	ErrUnsupportedPeriod = errors.New("vipdoc: unsupported minute period")
	ErrUnsupportedFormat = errors.New("vipdoc: unsupported format")
)

type ParseError struct {
	Path     string
	Offset   int64
	Expected int
	Actual   int
	Err      error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("vipdoc parse error: path=%s offset=%d expected=%d actual=%d: %v",
		e.Path, e.Offset, e.Expected, e.Actual, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func newParseError(path string, offset int64, expected int, actual int, err error) *ParseError {
	return &ParseError{
		Path:     path,
		Offset:   offset,
		Expected: expected,
		Actual:   actual,
		Err:      err,
	}
}

type UnsupportedError struct {
	Path    string
	Period  MinutePeriod
	Details string
	Err     error
}

func (e *UnsupportedError) Error() string {
	context := ""
	if e.Path != "" {
		context = fmt.Sprintf("path=%s", e.Path)
	}
	if e.Period != 0 {
		if context != "" {
			context += " "
		}
		context += fmt.Sprintf("period=%s", e.Period)
	}
	if context == "" {
		return fmt.Sprintf("vipdoc unsupported: %s: %v", e.Details, e.Err)
	}
	return fmt.Sprintf("vipdoc unsupported: %s: %s: %v", context, e.Details, e.Err)
}

func (e *UnsupportedError) Unwrap() error {
	return e.Err
}
