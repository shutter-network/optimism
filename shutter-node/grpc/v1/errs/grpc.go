package errs

import (
	"errors"

	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

var _ GRPCError = errr{}

type GRPCError interface {
	error
	GRPCStatus() *status.Status
}

func Error(err error) errr {
	// FIXME: edgecase err ==nil!
	// we don't handle nil errors in the Error()
	// etc. method.

	// TODO: from internal to external:
	// if a specific internal error is
	// passed in here (errors.Is()...),
	// this will get translated to an external
	// errror with specific status.
	return errr{err: err}
}

type errr struct {
	err error
}

var (
	errorInactive        = errors.New("shutter inactive")
	errorConnectionClose = errors.New("connection closed")
	errorCanceled        = errors.New("request canceled by client")
)

func (e *errr) statusUnknown() *status.Status {
	return status.New(codes.Unknown, e.Error())
}

func (e *errr) statusConectionClose() *status.Status {
	return status.New(codes.Unavailable, e.Error())
}

func (e *errr) statusCanceled() *status.Status {
	return status.New(codes.Canceled, e.Error())
}

func (e *errr) statusInactive() *status.Status {
	st := status.New(codes.FailedPrecondition, e.Error())
	ds, err := st.WithDetails(
		&epb.PreconditionFailure{
			Violations: []*epb.PreconditionFailure_Violation{
				{
					Type:        "Contract",
					Subject:     "Inbox",
					Description: "Shutter inactive for requested block",
				},
			},
		},
	)
	if err != nil {
		return st
	}
	return ds
}

func (s errr) GRPCStatus() *status.Status {
	if errors.Is(s, errorInactive) {
		return s.statusInactive()
	} else if errors.Is(s, errorConnectionClose) {
		return s.statusConectionClose()
	} else if errors.Is(s, errorCanceled) {
		return s.statusCanceled()
	} else {
		return s.statusUnknown()
	}
}

func (s errr) Error() string {
	// return the unwrapped string,
	// no the grpc-status string
	return s.err.Error()
}
