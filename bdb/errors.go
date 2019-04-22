package bdb

import (
	"fmt"
	"github.com/go-errors/errors"
)

type TemporaryChannelFailureError struct {
	ChanId  ChanId
	AmtMsat int64
}

func (err TemporaryChannelFailureError) Error() string {
	return fmt.Sprintf("Temporary channel failure occured for channel %v", err.ChanId)
}

type FeeInsufficientError struct {
	ChanId ChanId
}

func (err FeeInsufficientError) Error() string {
	return fmt.Sprintf("Insufficient fee for channel %v", err.ChanId)
}

type ExpiryTooSoonError struct {
	ChanId ChanId
}

func (err ExpiryTooSoonError) Error() string {
	return fmt.Sprintf("Expiry too soon for channel %v", err.ChanId)
}

// invoice is already paid
var InvoiceAlreadyPaidError = errors.New("Invoice is already paid")