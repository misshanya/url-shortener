package errorz

import "errors"

var (
	ErrTopLocked = errors.New("top is locked")
)
