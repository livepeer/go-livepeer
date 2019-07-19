package errors

// Error is an interface that describes methods for a PM related error that
// may be acceptable depending on the type of underlying error
type AcceptableError interface {
	error

	// Acceptable returns whether the error is acceptable
	Acceptable() bool
}

type acceptableError struct {
	err        error
	acceptable bool
}

func NewAcceptableError(err error, acceptable bool) *acceptableError {
	return &acceptableError{
		err:        err,
		acceptable: acceptable,
	}
}

// Error returns the underlying error as a string
func (re *acceptableError) Error() string {
	return re.err.Error()
}

// Acceptable returns whether the error is acceptable
func (re *acceptableError) Acceptable() bool {
	return re.acceptable
}
