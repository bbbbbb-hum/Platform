package mq

import "errors"

var (
	// ErrNatsNotEnabled NATS 未启用
	ErrNatsNotEnabled = errors.New("NATS is not enabled")

	// ErrJetStreamNotAvailable JetStream 上下文不可用
	ErrJetStreamNotAvailable = errors.New("JetStream context is not available")
)
