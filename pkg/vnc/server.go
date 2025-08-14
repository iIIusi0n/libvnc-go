// Package vnc provides VNC (Virtual Network Computing) client and server functionality
// by wrapping the native libvncserver and libvncclient C libraries.
//
// This package supports both VNC client and server operations, providing idiomatic
// Go interfaces for VNC protocol handling, framebuffer management, and event processing.
//
// Example usage:
//
//	// Create a VNC client
//	client := vnc.NewClient(8, 3, 4)
//	client.SetHost("127.0.0.1")
//	client.SetPort(5900)
//	client.SetPassword("password")
//	if !client.Init() {
//	    log.Fatal("Failed to initialize client")
//	}
//
//	// Create a VNC server
//	server := vnc.NewServer(800, 600, 8, 3, 4)
//	server.SetPort(5900)
//	server.SetPassword("password")
//	if err := server.InitServer(); err != nil {
//	    log.Fatal("Failed to initialize server")
//	}
package vnc

/*
#cgo LDFLAGS: -lvncserver
#include <rfb/rfb.h>
#include <stdlib.h>
#include <string.h>

extern void goKeyEventCallback(rfbBool down, rfbKeySym key, rfbClientPtr cl);
extern void goPointerEventCallback(int buttonMask, int x, int y, rfbClientPtr cl);
extern enum rfbNewClientAction goNewClientCallback(rfbClientPtr cl);
static inline void setKeyEventCallback(rfbScreenInfoPtr screen) {
    screen->kbdAddEvent = goKeyEventCallback;
}

static inline void setPointerEventCallback(rfbScreenInfoPtr screen) {
    screen->ptrAddEvent = goPointerEventCallback;
}

static inline void setNewClientCallback(rfbScreenInfoPtr screen) {
    screen->newClientHook = goNewClientCallback;
}

static inline void setServerPassword(rfbScreenInfoPtr screen, char* password) {
    char** passwords = malloc(2 * sizeof(char*));
    passwords[0] = strdup(password);
    passwords[1] = NULL;
    screen->authPasswdData = passwords;
    screen->passwordCheck = rfbCheckPasswordByList;
}

static inline void markRectAsModified(rfbScreenInfoPtr screen, int x, int y, int w, int h) {
    rfbMarkRectAsModified(screen, x, y, x + w, y + h);
}
*/
import "C"
import (
	"sync"
	"unsafe"
)

// KeyEventHandler is a callback function type for handling keyboard events.
// It receives the key state (down/up), key code, and client pointer.
type KeyEventHandler func(down bool, key uint32, clientPtr unsafe.Pointer)

// PointerEventHandler is a callback function type for handling mouse/pointer events.
// It receives the button mask, coordinates, and client pointer.
type PointerEventHandler func(buttonMask, x, y int, clientPtr unsafe.Pointer)

// NewClientHandler is a callback function type for handling new client connections.
// It receives the client pointer when a new client connects.
type NewClientHandler func(clientPtr unsafe.Pointer)

var (
	serverHandlers = make(map[*C.rfbScreenInfo]*Server)
	serverMutex    sync.RWMutex
)

//export goKeyEventCallback
func goKeyEventCallback(down C.rfbBool, key C.rfbKeySym, cl C.rfbClientPtr) {
	serverMutex.RLock()
	var server *Server
	for screen, srv := range serverHandlers {
		if cl != nil {
			// Find the server that owns this client
			clientIter := screen.clientHead
			for clientIter != nil {
				if clientIter == cl {
					server = srv
					break
				}
				clientIter = clientIter.next
			}
			if server != nil {
				break
			}
		}
	}
	serverMutex.RUnlock()

	if server != nil && server.keyEventHandler != nil {
		server.keyEventHandler(down != 0, uint32(key), unsafe.Pointer(cl))
	}
}

//export goPointerEventCallback
func goPointerEventCallback(buttonMask C.int, x C.int, y C.int, cl C.rfbClientPtr) {
	serverMutex.RLock()
	var server *Server
	for screen, srv := range serverHandlers {
		if cl != nil {
			// Find the server that owns this client
			clientIter := screen.clientHead
			for clientIter != nil {
				if clientIter == cl {
					server = srv
					break
				}
				clientIter = clientIter.next
			}
			if server != nil {
				break
			}
		}
	}
	serverMutex.RUnlock()

	if server != nil && server.pointerEventHandler != nil {
		server.pointerEventHandler(int(buttonMask), int(x), int(y), unsafe.Pointer(cl))
	}
}

//export goNewClientCallback
func goNewClientCallback(cl C.rfbClientPtr) C.enum_rfbNewClientAction {
	serverMutex.RLock()
	var server *Server
	for screen, srv := range serverHandlers {
		if cl != nil {
			// Find the server that owns this client
			clientIter := screen.clientHead
			for clientIter != nil {
				if clientIter == cl {
					server = srv
					break
				}
				clientIter = clientIter.next
			}
			if server != nil {
				break
			}
		}
	}
	serverMutex.RUnlock()

	if server != nil && server.newClientHandler != nil {
		server.newClientHandler(unsafe.Pointer(cl))
	}

	return C.RFB_CLIENT_ACCEPT
}

// Server represents a VNC server that can accept client connections.
// It provides methods for creating servers, handling client events,
// and managing framebuffer content.
type Server struct {
	rfbScreen           *C.rfbScreenInfo
	frameBuffer         []byte
	keyEventHandler     KeyEventHandler
	pointerEventHandler PointerEventHandler
	newClientHandler    NewClientHandler
	passwordCString     *C.char
	running             bool
}

// NewServer creates a new VNC server with the specified dimensions and pixel format.
//
// Parameters:
//   - width: Screen width in pixels
//   - height: Screen height in pixels
//   - bitsPerSample: Number of bits per color sample (typically 8)
//   - samplesPerPixel: Number of color samples per pixel (typically 3 for RGB)
//   - bytesPerPixel: Number of bytes per pixel (typically 4 for RGBA)
//
// Returns a new Server instance or nil if creation fails.
func NewServer(width, height, bitsPerSample, samplesPerPixel, bytesPerPixel int) *Server {
	screen := C.rfbGetScreen(nil, nil, C.int(width), C.int(height), C.int(bitsPerSample), C.int(samplesPerPixel), C.int(bytesPerPixel))
	if screen == nil {
		return nil
	}

	bufferSize := width * height * bytesPerPixel
	frameBuffer := make([]byte, bufferSize)
	screen.frameBuffer = (*C.char)(unsafe.Pointer(&frameBuffer[0]))

	server := &Server{
		rfbScreen:   screen,
		frameBuffer: frameBuffer,
		running:     false,
	}

	serverMutex.Lock()
	serverHandlers[screen] = server
	serverMutex.Unlock()

	return server
}

func (s *Server) SetPixelFormat(format PixelFormat) {
	s.rfbScreen.serverFormat.bitsPerPixel = C.uchar(format.BitsPerPixel)
	s.rfbScreen.serverFormat.depth = C.uchar(format.Depth)
	if format.BigEndian {
		s.rfbScreen.serverFormat.bigEndian = 1
	} else {
		s.rfbScreen.serverFormat.bigEndian = 0
	}
	if format.TrueColour {
		s.rfbScreen.serverFormat.trueColour = 1
	} else {
		s.rfbScreen.serverFormat.trueColour = 0
	}
	s.rfbScreen.serverFormat.redMax = C.ushort(format.RedMax)
	s.rfbScreen.serverFormat.greenMax = C.ushort(format.GreenMax)
	s.rfbScreen.serverFormat.blueMax = C.ushort(format.BlueMax)
	s.rfbScreen.serverFormat.redShift = C.uchar(format.RedShift)
	s.rfbScreen.serverFormat.greenShift = C.uchar(format.GreenShift)
	s.rfbScreen.serverFormat.blueShift = C.uchar(format.BlueShift)
}

func (s *Server) SetStandardPixelFormat() {
	s.SetPixelFormat(PixelFormatStandard)
}

func (s *Server) SetPort(port int) {
	s.rfbScreen.port = C.int(port)
}

func (s *Server) SetPassword(password string) {
	if s.passwordCString != nil {
		C.free(unsafe.Pointer(s.passwordCString))
	}
	s.passwordCString = C.CString(password)
	C.setServerPassword(s.rfbScreen, s.passwordCString)
}

func (s *Server) SetKeyEventHandler(handler KeyEventHandler) {
	s.keyEventHandler = handler
	C.setKeyEventCallback(s.rfbScreen)
}

func (s *Server) SetPointerEventHandler(handler PointerEventHandler) {
	s.pointerEventHandler = handler
	C.setPointerEventCallback(s.rfbScreen)
}

func (s *Server) SetNewClientHandler(handler NewClientHandler) {
	s.newClientHandler = handler
	C.setNewClientCallback(s.rfbScreen)
}

func (s *Server) GetFrameBuffer() []byte {
	return s.frameBuffer
}

func (s *Server) GetWidth() int {
	return int(s.rfbScreen.width)
}

func (s *Server) GetHeight() int {
	return int(s.rfbScreen.height)
}

func (s *Server) MarkRectAsModified(x, y, w, h int) {
	C.markRectAsModified(s.rfbScreen, C.int(x), C.int(y), C.int(w), C.int(h))
}

func (s *Server) InitServer() error {
	C.rfbInitServer(s.rfbScreen)
	s.running = true
	return nil
}

func (s *Server) ProcessEvents(timeoutMs int) {
	C.rfbProcessEvents(s.rfbScreen, C.long(timeoutMs*1000)) // Convert to microseconds
}

func (s *Server) RunEventLoop(timeoutMs int) error {
	for s.running {
		s.ProcessEvents(timeoutMs)
	}
	return nil
}

func (s *Server) IsActive() bool {
	return C.rfbIsActive(s.rfbScreen) != 0
}

func (s *Server) GetClientCount() int {
	count := 0
	client := s.rfbScreen.clientHead
	for client != nil {
		count++
		client = client.next
	}
	return count
}

func (s *Server) Stop() {
	s.running = false
}

func (s *Server) Close() {
	s.Stop()

	serverMutex.Lock()
	delete(serverHandlers, s.rfbScreen)
	serverMutex.Unlock()

	if s.passwordCString != nil {
		C.free(unsafe.Pointer(s.passwordCString))
		s.passwordCString = nil
	}

	if s.rfbScreen != nil {
		C.rfbScreenCleanup(s.rfbScreen)
		s.rfbScreen = nil
	}
}
