package vnc

import "unsafe"

type GotFrameBufferUpdateHandler func(x, y, w, h int)
type FinishedFrameBufferUpdateHandler func()

type KeyEventHandler func(down bool, key uint32, clientPtr unsafe.Pointer)
type PointerEventHandler func(buttonMask, x, y int, clientPtr unsafe.Pointer)
type NewClientHandler func(clientPtr unsafe.Pointer)
