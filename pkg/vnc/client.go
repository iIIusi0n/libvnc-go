package vnc

/*
#cgo LDFLAGS: -lvncclient
#include <rfb/rfbclient.h>
#include <stdlib.h>
#include <string.h>

extern void goGotFrameBufferUpdateCallback(rfbClient* cl, int x, int y, int w, int h);

static inline void setGotFrameBufferUpdateCallback(rfbClient* cl) {
    cl->GotFrameBufferUpdate = goGotFrameBufferUpdateCallback;
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

var (
	clientHandlers = make(map[*C.rfbClient]GotFrameBufferUpdateHandler)
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

type Client struct {
	rfbClient *C.rfbClient
	gotFrameBufferUpdateHandler GotFrameBufferUpdateHandler
	hostCString *C.char 
	passwordCString *C.char
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

func (c *Client) Close() {
	clientMutex.Lock()
	delete(clientHandlers, c.rfbClient)
	clientMutex.Unlock()
	
	if c.hostCString != nil {
		C.free(unsafe.Pointer(c.hostCString))
		c.hostCString = nil
	}
	
	if c.passwordCString != nil {
		C.free(unsafe.Pointer(c.passwordCString))
		c.passwordCString = nil
	}
}
