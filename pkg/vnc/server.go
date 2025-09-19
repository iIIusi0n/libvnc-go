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

type Server struct {
	rfbScreen           *C.rfbScreenInfo
	frameBuffer         []byte
	keyEventHandler     KeyEventHandler
	pointerEventHandler PointerEventHandler
	newClientHandler    NewClientHandler
	running             bool
}

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
	C.setServerPassword(s.rfbScreen, C.CString(password))
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
	C.rfbProcessEvents(s.rfbScreen, C.long(timeoutMs*1000))
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

	if s.rfbScreen != nil {
		C.rfbScreenCleanup(s.rfbScreen)
		s.rfbScreen = nil
	}
}
