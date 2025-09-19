package vnc

type ClientPort interface {
	SetHost(host string)
	SetPort(port int)
	SetPassword(password string)
	SetStandardPixelFormat()
	Init() bool
	RunEventLoop(timeoutMs int) error
	IsConnected() bool
	Close()

	GetFrameBufferWidth() int
	GetFrameBufferHeight() int
	GetFrameBuffer() []byte
	SendFrameBufferUpdateRequest(x, y, w, h int, incremental bool)

	SendPointerEvent(x, y int, buttonMask uint8)
	SendKeyEvent(key uint32, down bool)

	SetGotFrameBufferUpdateHandler(handler GotFrameBufferUpdateHandler)
}

type ServerPort interface {
	SetPort(port int)
	SetStandardPixelFormat()
	InitServer() error
	RunEventLoop(timeoutMs int) error
	Close()

	// Stop signals the server main loop to stop processing events.
	Stop()

	GetFrameBuffer() []byte
	GetWidth() int
	GetHeight() int
	MarkRectAsModified(x, y, w, h int)

	SetPointerEventHandler(handler PointerEventHandler)
	SetKeyEventHandler(handler KeyEventHandler)
}

type ClientFactory func(bitsPerSample, samplesPerPixel, bytesPerPixel int) (ClientPort, error)
type ServerFactory func(width, height, bitsPerSample, samplesPerPixel, bytesPerPixel int) (ServerPort, error)

var defaultClientFactory ClientFactory
var defaultServerFactory ServerFactory
