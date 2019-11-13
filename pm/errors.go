package pm

type receiveError struct {
	err        error
	acceptable bool
}

func newReceiveError(err error, acceptable bool) *receiveError {
	return &receiveError{
		err:        err,
		acceptable: acceptable,
	}
}

// Error returns the underlying error as a string
func (re *receiveError) Error() string {
	return re.err.Error()
}

// Acceptable returns whether the error is acceptable
func (re *receiveError) Acceptable() bool {
	return re.acceptable
}
