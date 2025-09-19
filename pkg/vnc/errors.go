package vnc

import "errors"

var (
	ErrCreateClient = errors.New("failed to create VNC client")
	ErrCreateServer = errors.New("failed to create VNC server")
)
