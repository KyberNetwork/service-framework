package client

import (
	"github.com/cenkalti/backoff/v4"
)

// BackoffCfg is a hotcfg to create a backoff.ExponentialBackOff
type BackoffCfg struct {
	backoff.ExponentialBackOff `mapstructure:",squash"`
	MaxRetries                 uint64
	backoff.BackOff
}

func (b *BackoffCfg) OnUpdate(_, new *BackoffCfg) {
	expBackoff := backoff.NewExponentialBackOff()
	if new.InitialInterval != 0 {
		expBackoff.InitialInterval = new.InitialInterval
	}
	if new.RandomizationFactor != 0 {
		expBackoff.RandomizationFactor = new.RandomizationFactor
	}
	if new.Multiplier != 0 {
		expBackoff.Multiplier = new.Multiplier
	}
	if new.MaxInterval != 0 {
		expBackoff.MaxInterval = new.MaxInterval
	}
	if new.MaxElapsedTime != 0 {
		expBackoff.MaxElapsedTime = new.MaxElapsedTime
	}
	expBackoff.Reset()
	new.BackOff = expBackoff
	if new.MaxRetries != 0 {
		new.BackOff = backoff.WithMaxRetries(expBackoff, new.MaxRetries)
	}
}

func (b *BackoffCfg) Retry(o backoff.Operation) error {
	return backoff.Retry(o, b.BackOff)
}

func (b *BackoffCfg) RetryNotify(o backoff.Operation, n backoff.Notify) error {
	return backoff.RetryNotify(o, b.BackOff, n)
}
