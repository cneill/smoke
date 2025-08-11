package tools

import "errors"

var (
	ErrArguments   = errors.New("arguments error")
	ErrCallFailed  = errors.New("tool call failed")
	ErrUnknownTool = errors.New("unknown tool")
	ErrFileSystem  = errors.New("file system error")
)
