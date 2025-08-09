package repository

import "errors"

var (
	ErrPlayerCountTooHigh  = errors.New("player count too high")
	ErrPlayerAlreadyExists = errors.New("player already exists")
)
