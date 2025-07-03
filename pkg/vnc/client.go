package vnc

/*
#cgo LDFLAGS: -lvncclient
#include <rfb/rfbclient.h>
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
	"sync"
	"unsafe"
)

type GotFrameBufferUpdateHandler func(x, y, w, h int)
type FinishedFrameBufferUpdateHandler func()

var (
	clientHandlers = make(map[*C.rfbClient]GotFrameBufferUpdateHandler)
	clientFinishedHandlers = make(map[*C.rfbClient]FinishedFrameBufferUpdateHandler)
	clientMutex    sync.RWMutex
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
	rfbClient *C.rfbClient
	gotFrameBufferUpdateHandler GotFrameBufferUpdateHandler
	finishedFrameBufferUpdateHandler FinishedFrameBufferUpdateHandler
	hostCString *C.char 
	passwordCString *C.char
	encodingsCString *C.char
}

func NewClient(bitsPerSample, samplesPerPixel, bytesPerPixel int) *Client {
	rfbClient := C.rfbGetClient(C.int(bitsPerSample), C.int(samplesPerPixel), C.int(bytesPerPixel))
	if rfbClient == nil {
		return nil
	}
	return &Client{rfbClient: rfbClient}
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

func (c *Client) SetPixelFormat(bitsPerPixel, depth int, bigEndian, trueColour bool, redMax, greenMax, blueMax, redShift, greenShift, blueShift int) {
	c.rfbClient.format.bitsPerPixel = C.uchar(bitsPerPixel)
	c.rfbClient.format.depth = C.uchar(depth)
	if bigEndian {
		c.rfbClient.format.bigEndian = 1
	} else {
		c.rfbClient.format.bigEndian = 0
	}
	if trueColour {
		c.rfbClient.format.trueColour = 1
	} else {
		c.rfbClient.format.trueColour = 0
	}
	c.rfbClient.format.redMax = C.ushort(redMax)
	c.rfbClient.format.greenMax = C.ushort(greenMax)
	c.rfbClient.format.blueMax = C.ushort(blueMax)
	c.rfbClient.format.redShift = C.uchar(redShift)
	c.rfbClient.format.greenShift = C.uchar(greenShift)
	c.rfbClient.format.blueShift = C.uchar(blueShift)
}

func (c *Client) SetIPEPSPixelFormat() {
	c.SetPixelFormat(16, 15, false, true, 31, 31, 31, 10, 5, 0)
}

func (c *Client) SetStandardPixelFormat() {
	c.SetPixelFormat(32, 24, false, true, 255, 255, 255, 0, 8, 16)
}

func (c *Client) SetAppData(compressLevel, qualityLevel int, encodings string, useRemoteCursor bool) {
	c.rfbClient.appData.compressLevel = C.int(compressLevel)
	c.rfbClient.appData.qualityLevel = C.int(qualityLevel)
	
	if c.encodingsCString != nil {
		C.free(unsafe.Pointer(c.encodingsCString))
	}
	c.encodingsCString = C.CString(encodings)
	c.rfbClient.appData.encodingsString = c.encodingsCString
	
	if useRemoteCursor {
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
	bufferSize := int(c.rfbClient.width) * int(c.rfbClient.height) * int(c.rfbClient.format.bitsPerPixel / 8)
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
