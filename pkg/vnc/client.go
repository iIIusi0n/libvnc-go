package vnc

/*
#cgo LDFLAGS: -lvncclient -lvncserver
#include <rfb/rfbclient.h>
#include <rfb/rfb.h>
#include <stdlib.h>
#include <string.h>

extern void goGotFrameBufferUpdateCallback(rfbClient* cl, int x, int y, int w, int h);
extern void goFinishedFrameBufferUpdateCallback(rfbClient* cl);

static inline void setGotFrameBufferUpdateCallback(rfbClient* cl) {
    cl->GotFrameBufferUpdate = goGotFrameBufferUpdateCallback;
}

static inline void setFinishedFrameBufferUpdateCallback(rfbClient* cl) {
    cl->FinishedFrameBufferUpdate = goFinishedFrameBufferUpdateCallback;
}

static char* stored_password = NULL;

static char* passwordCallback(rfbClient* cl) {
    return stored_password;
}

static inline void setPassword(rfbClient* cl, char* password) {
    if (stored_password != NULL) {
        free(stored_password);
    }
    stored_password = strdup(password);
    cl->GetPassword = passwordCallback;
}
*/
import "C"
import (
	"fmt"
	"net"
	"os"
	"sync"
	"unsafe"
)

type GotFrameBufferUpdateHandler func(x, y, w, h int)
type FinishedFrameBufferUpdateHandler func()

type PixelFormat struct {
	BitsPerPixel int
	Depth        int
	BigEndian    bool
	TrueColour   bool
	RedMax       int
	GreenMax     int
	BlueMax      int
	RedShift     int
	GreenShift   int
	BlueShift    int
}

type AppDataConfig struct {
	CompressLevel   int
	QualityLevel    int
	Encodings       string
	UseRemoteCursor bool
}

// Predefined pixel formats
var (
	PixelFormatBGR0 = PixelFormat{
		BitsPerPixel: 32, Depth: 24, BigEndian: false, TrueColour: true,
		RedMax: 255, GreenMax: 255, BlueMax: 255,
		RedShift: 16, GreenShift: 8, BlueShift: 0,
	}

	PixelFormatStandard = PixelFormat{
		BitsPerPixel: 32, Depth: 24, BigEndian: false, TrueColour: true,
		RedMax: 255, GreenMax: 255, BlueMax: 255,
		RedShift: 0, GreenShift: 8, BlueShift: 16,
	}

	PixelFormatIPEPS = PixelFormat{
		BitsPerPixel: 16, Depth: 15, BigEndian: false, TrueColour: true,
		RedMax: 31, GreenMax: 31, BlueMax: 31,
		RedShift: 10, GreenShift: 5, BlueShift: 0,
	}
)

var (
	clientHandlers         = make(map[*C.rfbClient]GotFrameBufferUpdateHandler)
	clientFinishedHandlers = make(map[*C.rfbClient]FinishedFrameBufferUpdateHandler)
	clientMutex            sync.RWMutex
)

//export goGotFrameBufferUpdateCallback
func goGotFrameBufferUpdateCallback(cl *C.rfbClient, x, y, w, h C.int) {
	clientMutex.RLock()
	handler, exists := clientHandlers[cl]
	clientMutex.RUnlock()

	if exists && handler != nil {
		handler(int(x), int(y), int(w), int(h))
	}
}

//export goFinishedFrameBufferUpdateCallback
func goFinishedFrameBufferUpdateCallback(cl *C.rfbClient) {
	clientMutex.RLock()
	handler, exists := clientFinishedHandlers[cl]
	clientMutex.RUnlock()

	if exists && handler != nil {
		handler()
	}
}

type Client struct {
	rfbClient                        *C.rfbClient
	gotFrameBufferUpdateHandler      GotFrameBufferUpdateHandler
	finishedFrameBufferUpdateHandler FinishedFrameBufferUpdateHandler
	hostCString                      *C.char
	passwordCString                  *C.char
	encodingsCString                 *C.char
}

type ServerClient struct {
	rfbClient C.rfbClientPtr
	connFile  *os.File
}

func NewClient(bitsPerSample, samplesPerPixel, bytesPerPixel int) *Client {
	rfbClient := C.rfbGetClient(C.int(bitsPerSample), C.int(samplesPerPixel), C.int(bytesPerPixel))
	if rfbClient == nil {
		return nil
	}
	return &Client{rfbClient: rfbClient}
}

func NewClientWithConn(screen *Server, conn net.Conn) (*ServerClient, error) {
	if screen == nil {
		return nil, fmt.Errorf("screen cannot be nil")
	}
	if conn == nil {
		return nil, fmt.Errorf("conn cannot be nil")
	}

	var fd int
	var file *os.File
	switch c := conn.(type) {
	case *net.TCPConn:
		var err error
		file, err = c.File()
		if err != nil {
			return nil, fmt.Errorf("failed to get file descriptor from TCPConn: %v", err)
		}
		fd = int(file.Fd())
	case *net.UnixConn:
		var err error
		file, err = c.File()
		if err != nil {
			return nil, fmt.Errorf("failed to get file descriptor from UnixConn: %v", err)
		}
		fd = int(file.Fd())
	default:
		return nil, fmt.Errorf("unsupported connection type: %T", conn)
	}

	rfbClient := C.rfbNewClient(screen.rfbScreen, C.int(fd))
	if rfbClient == nil {
		file.Close()
		return nil, fmt.Errorf("failed to create RFB client")
	}

	return &ServerClient{rfbClient: rfbClient, connFile: file}, nil
}

func (sc *ServerClient) GetPointer() unsafe.Pointer {
	return unsafe.Pointer(sc.rfbClient)
}

func (sc *ServerClient) Close() {
	if sc.rfbClient != nil {
		sc.rfbClient = nil
	}
	if sc.connFile != nil {
		sc.connFile = nil
	}
}

func (c *Client) SetGotFrameBufferUpdateHandler(handler GotFrameBufferUpdateHandler) {
	c.gotFrameBufferUpdateHandler = handler

	clientMutex.Lock()
	clientHandlers[c.rfbClient] = handler
	clientMutex.Unlock()

	C.setGotFrameBufferUpdateCallback(c.rfbClient)
}

func (c *Client) SetFinishedFrameBufferUpdateHandler(handler FinishedFrameBufferUpdateHandler) {
	c.finishedFrameBufferUpdateHandler = handler

	clientMutex.Lock()
	clientFinishedHandlers[c.rfbClient] = handler
	clientMutex.Unlock()

	C.setFinishedFrameBufferUpdateCallback(c.rfbClient)
}

func (c *Client) SetHost(host string) {
	if c.hostCString != nil {
		C.free(unsafe.Pointer(c.hostCString))
	}
	c.hostCString = C.CString(host)
	c.rfbClient.serverHost = c.hostCString
}

func (c *Client) SetPort(port int) {
	c.rfbClient.serverPort = C.int(port)
}

func (c *Client) SetPassword(password string) {
	if c.passwordCString != nil {
		C.free(unsafe.Pointer(c.passwordCString))
	}
	c.passwordCString = C.CString(password)
	C.setPassword(c.rfbClient, c.passwordCString)
}

func (c *Client) SetPixelFormat(format PixelFormat) {
	c.rfbClient.format.bitsPerPixel = C.uchar(format.BitsPerPixel)
	c.rfbClient.format.depth = C.uchar(format.Depth)
	if format.BigEndian {
		c.rfbClient.format.bigEndian = 1
	} else {
		c.rfbClient.format.bigEndian = 0
	}
	if format.TrueColour {
		c.rfbClient.format.trueColour = 1
	} else {
		c.rfbClient.format.trueColour = 0
	}
	c.rfbClient.format.redMax = C.ushort(format.RedMax)
	c.rfbClient.format.greenMax = C.ushort(format.GreenMax)
	c.rfbClient.format.blueMax = C.ushort(format.BlueMax)
	c.rfbClient.format.redShift = C.uchar(format.RedShift)
	c.rfbClient.format.greenShift = C.uchar(format.GreenShift)
	c.rfbClient.format.blueShift = C.uchar(format.BlueShift)
}

func (c *Client) SetIPEPSPixelFormat() {
	c.SetPixelFormat(PixelFormatIPEPS)
}

func (c *Client) SetStandardPixelFormat() {
	c.SetPixelFormat(PixelFormatStandard)
}

func (c *Client) SetBGR0PixelFormat() {
	c.SetPixelFormat(PixelFormatBGR0)
}

func (c *Client) SetAppData(config AppDataConfig) {
	c.rfbClient.appData.compressLevel = C.int(config.CompressLevel)
	c.rfbClient.appData.qualityLevel = C.int(config.QualityLevel)

	if c.encodingsCString != nil {
		C.free(unsafe.Pointer(c.encodingsCString))
	}
	c.encodingsCString = C.CString(config.Encodings)
	c.rfbClient.appData.encodingsString = c.encodingsCString

	if config.UseRemoteCursor {
		c.rfbClient.appData.useRemoteCursor = C.rfbBool(1)
	} else {
		c.rfbClient.appData.useRemoteCursor = C.rfbBool(0)
	}
}

func (c *Client) SetCanHandleNewFBSize(canHandle bool) {
	if canHandle {
		c.rfbClient.canHandleNewFBSize = C.int(1)
	} else {
		c.rfbClient.canHandleNewFBSize = C.int(0)
	}
}

func (c *Client) Init() bool {
	return C.rfbInitClient(c.rfbClient, nil, nil) != 0
}

func (c *Client) WaitForMessage(timeoutMs int) int {
	return int(C.WaitForMessage(c.rfbClient, C.uint(timeoutMs)))
}

func (c *Client) HandleRFBServerMessage() bool {
	return C.HandleRFBServerMessage(c.rfbClient) != 0
}

func (c *Client) RunEventLoop(timeoutMs int) error {
	for {
		result := c.WaitForMessage(timeoutMs)

		if result < 0 {
			if result == -1 {
				return fmt.Errorf("connection lost or server disconnected (WaitForMessage returned -1)")
			}
			return fmt.Errorf("error waiting for message: %d", result)
		}

		if result > 0 {
			if !c.HandleRFBServerMessage() {
				return fmt.Errorf("error handling server message - connection may have been lost")
			}
		}
	}
}

func (c *Client) RunEventLoopWithContext(done <-chan struct{}, timeoutMs int) error {
	for {
		select {
		case <-done:
			return nil
		default:
			result := c.WaitForMessage(timeoutMs)

			if result < 0 {
				return fmt.Errorf("error waiting for message: %d", result)
			}

			if result > 0 {
				if !c.HandleRFBServerMessage() {
					return fmt.Errorf("error handling server message")
				}
			}
		}
	}
}

func (c *Client) GetFrameBufferWidth() int {
	return int(c.rfbClient.width)
}

func (c *Client) GetFrameBufferHeight() int {
	return int(c.rfbClient.height)
}

func (c *Client) GetFrameBuffer() []byte {
	if c.rfbClient.frameBuffer == nil {
		return nil
	}
	bufferSize := int(c.rfbClient.width) * int(c.rfbClient.height) * int(c.rfbClient.format.bitsPerPixel/8)
	return C.GoBytes(unsafe.Pointer(c.rfbClient.frameBuffer), C.int(bufferSize))
}

func (c *Client) SendFrameBufferUpdateRequest(x, y, w, h int, incremental bool) {
	var inc C.rfbBool
	if incremental {
		inc = 1
	} else {
		inc = 0
	}
	C.SendFramebufferUpdateRequest(c.rfbClient, C.int(x), C.int(y), C.int(w), C.int(h), inc)
}

func (c *Client) SendPointerEvent(x, y int, buttonMask uint8) {
	C.SendPointerEvent(c.rfbClient, C.int(x), C.int(y), C.int(buttonMask))
}

func (c *Client) SendKeyEvent(key uint32, down bool) {
	var d C.rfbBool
	if down {
		d = 1
	} else {
		d = 0
	}
	C.SendKeyEvent(c.rfbClient, C.uint(key), d)
}

func (c *Client) Close() {
	clientMutex.Lock()
	delete(clientHandlers, c.rfbClient)
	delete(clientFinishedHandlers, c.rfbClient)
	clientMutex.Unlock()

	if c.hostCString != nil {
		C.free(unsafe.Pointer(c.hostCString))
		c.hostCString = nil
	}

	if c.passwordCString != nil {
		C.free(unsafe.Pointer(c.passwordCString))
		c.passwordCString = nil
	}

	if c.encodingsCString != nil {
		C.free(unsafe.Pointer(c.encodingsCString))
		c.encodingsCString = nil
	}

	if c.rfbClient != nil {
		C.rfbClientCleanup(c.rfbClient)
		c.rfbClient = nil
	}
}
