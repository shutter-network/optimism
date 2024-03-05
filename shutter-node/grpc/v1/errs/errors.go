package errs

var (
	Inactive         = Error(errorInactive)
	ConnectionClosed = Error(errorConnectionClose)
	Canceled         = Error(errorCanceled)
)
