package gepa

import "errors"

var (
	ErrScriptIDRequired = errors.New("script_id is required")
	ErrUnknownScriptID  = errors.New("unknown script_id")
)
