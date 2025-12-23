package errors

import "errors"

var (
	ErrDropMessage = errors.New("drop message")
	ErrDLQMessage  = errors.New("send to dlq")
)
