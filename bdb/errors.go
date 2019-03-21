package bdb

import "fmt"

type TemporaryChannelFailureError struct {
	ChanId  ChanId
	AmtMsat int64
}

func (err TemporaryChannelFailureError) Error() string {
	return fmt.Sprintf("Temporary channel failure occured for channel %v", err.ChanId)
}
