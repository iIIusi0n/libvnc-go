package vnc

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"sync"
	"time"
	"unsafe"
)

type Multiplexer struct {
	proxyServer ServerPort
	proxyClient ClientPort

	targetHost     string
	targetPort     int
	targetPassword string

	listenPort int

	isConnected         bool
	onConnectionOnline  func()
	onConnectionOffline func()

	clientFactory ClientFactory
	serverFactory ServerFactory

	// internal coordination helpers
	serverLoopStop chan struct{} // closes to stop current proxyServer event loop
	runningWG      sync.WaitGroup
}

func NewMultiplexer(targetHost string, targetPort int, targetPassword string, listenPort int, onConnectionOnline func(), onConnectionOffline func()) (*Multiplexer, error) {
	return NewMultiplexerWithFactories(targetHost, targetPort, targetPassword, listenPort, onConnectionOnline, onConnectionOffline, defaultClientFactory, defaultServerFactory)
}

func NewMultiplexerWithFactories(targetHost string, targetPort int, targetPassword string, listenPort int, onConnectionOnline func(), onConnectionOffline func(), clientFactory ClientFactory, serverFactory ServerFactory) (*Multiplexer, error) {
	m := &Multiplexer{
		targetHost:          targetHost,
		targetPort:          targetPort,
		targetPassword:      targetPassword,
		listenPort:          listenPort,
		onConnectionOnline:  onConnectionOnline,
		onConnectionOffline: onConnectionOffline,
		clientFactory:       clientFactory,
		serverFactory:       serverFactory,
		serverLoopStop:      make(chan struct{}),
	}

	if err := m.initProxyClient(clientFactory); err != nil {
		return nil, fmt.Errorf("failed to initialize proxy client: %w", err)
	}

	if err := m.initProxyServer(serverFactory); err != nil {
		return nil, fmt.Errorf("failed to initialize proxy server: %w", err)
	}

	m.setupHandlers()

	return m, nil
}

func (m *Multiplexer) initProxyClient(factory ClientFactory) error {
	if factory == nil {
		factory = defaultClientFactory
	}

	client, err := factory(8, 3, 4)
	if err != nil {
		return err
	}
	m.proxyClient = client

	m.proxyClient.SetHost(m.targetHost)
	m.proxyClient.SetPort(m.targetPort)
	if m.targetPassword != "" {
		m.proxyClient.SetPassword(m.targetPassword)
	}
	m.proxyClient.SetStandardPixelFormat()

	if !m.proxyClient.Init() {
		return fmt.Errorf("failed to initialize VNC client connection")
	}

	log.Println("Proxy client initialized and connected to target server.")

	width := m.proxyClient.GetFrameBufferWidth()
	height := m.proxyClient.GetFrameBufferHeight()
	m.proxyClient.SendFrameBufferUpdateRequest(0, 0, width, height, false)

	if !m.isConnected {
		m.isConnected = true
		if m.onConnectionOnline != nil {
			m.onConnectionOnline()
		}
	}

	return nil
}

func (m *Multiplexer) initProxyServer(factory ServerFactory) error {
	width := m.proxyClient.GetFrameBufferWidth()
	height := m.proxyClient.GetFrameBufferHeight()

	if factory == nil {
		factory = defaultServerFactory
	}

	server, err := factory(width, height, 8, 3, 4)
	if err != nil {
		return err
	}
	m.proxyServer = server

	m.proxyServer.SetPort(m.listenPort)
	m.proxyServer.SetStandardPixelFormat()

	if err := m.proxyServer.InitServer(); err != nil {
		return fmt.Errorf("failed to initialize VNC server: %w", err)
	}

	log.Printf("Proxy server initialized and listening on port %d.", m.listenPort)
	return nil
}

func (m *Multiplexer) setupHandlers() {
	m.proxyServer.SetPointerEventHandler(m.handlePointerEvent)
	m.proxyServer.SetKeyEventHandler(m.handleKeyEvent)

	m.proxyClient.SetGotFrameBufferUpdateHandler(m.handleFramebufferUpdate)
}

func (m *Multiplexer) handlePointerEvent(buttonMask, x, y int, clientPtr unsafe.Pointer) {
	if m.proxyClient.IsConnected() {
		m.proxyClient.SendPointerEvent(x, y, uint8(buttonMask))
	}
}

func (m *Multiplexer) handleKeyEvent(down bool, key uint32, clientPtr unsafe.Pointer) {
	if m.proxyClient.IsConnected() {
		m.proxyClient.SendKeyEvent(key, down)
	}
}

func (m *Multiplexer) handleFramebufferUpdate(x, y, w, h int) {
	if m.proxyServer == nil {
		return
	}

	clientFB := m.proxyClient.GetFrameBuffer()
	serverFB := m.proxyServer.GetFrameBuffer()
	if clientFB == nil || serverFB == nil {
		return
	}

	clientWidth := m.proxyClient.GetFrameBufferWidth()
	serverWidth := m.proxyServer.GetWidth()
	bytesPerPixel := 4

	for i := 0; i < h; i++ {
		clientStart := ((y+i)*clientWidth + x) * bytesPerPixel
		serverStart := ((y+i)*serverWidth + x) * bytesPerPixel

		clientEnd := clientStart + w*bytesPerPixel
		serverEnd := serverStart + w*bytesPerPixel

		copy(serverFB[serverStart:serverEnd], clientFB[clientStart:clientEnd])
	}

	m.proxyServer.MarkRectAsModified(x, y, w, h)
}

func (m *Multiplexer) drawDisconnectedScreen() {
	if m.proxyServer == nil {
		return
	}
	width := m.proxyServer.GetWidth()
	height := m.proxyServer.GetHeight()
	serverFB := m.proxyServer.GetFrameBuffer()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			offset := (y*width + x) * 4
			serverFB[offset] = byte(b >> 8)
			serverFB[offset+1] = byte(g >> 8)
			serverFB[offset+2] = byte(r >> 8)
			serverFB[offset+3] = byte(a >> 8)
		}
	}

	m.proxyServer.MarkRectAsModified(0, 0, width, height)
}

func (m *Multiplexer) GetRGBData() ([]byte, int, int) {
	if m.proxyServer == nil {
		return nil, 0, 0
	}

	serverFB := m.proxyServer.GetFrameBuffer()
	width := m.proxyServer.GetWidth()
	height := m.proxyServer.GetHeight()

	rgbData := make([]byte, width*height*3)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			fbOffset := (y*width + x) * 4
			rgbData[offset] = serverFB[fbOffset]
			rgbData[offset+1] = serverFB[fbOffset+1]
			rgbData[offset+2] = serverFB[fbOffset+2]
		}
	}

	return rgbData, width, height
}

// startProxyServerLoop launches the event loop for the currently configured
// proxyServer in a dedicated goroutine. The loop terminates when either the
// serverLoopStop channel is closed or the proxyServer.RunEventLoop returns.
func (m *Multiplexer) startProxyServerLoop() {
	if m.proxyServer == nil {
		return
	}

	// Ensure any previous loop is stopped
	m.stopProxyServerLoop()

	m.serverLoopStop = make(chan struct{})
	m.runningWG.Add(1)
	go func(srv ServerPort, stop <-chan struct{}) {
		defer m.runningWG.Done()
		log.Println("Proxy server event loop started.")
		doneCh := make(chan struct{})
		go func() {
			_ = srv.RunEventLoop(10) // blocks until srv.Stop() / Close()
			close(doneCh)
		}()

		select {
		case <-stop:
			srv.Stop()
		case <-doneCh:
			// server finished on its own
		}
		log.Println("Proxy server event loop stopped.")
	}(m.proxyServer, m.serverLoopStop)
}

// stopProxyServerLoop requests the currently running proxyServer event loop to
// stop and waits for the goroutine to terminate.
func (m *Multiplexer) stopProxyServerLoop() {
	if m.serverLoopStop != nil {
		select {
		case <-m.serverLoopStop:
		default:
			close(m.serverLoopStop)
		}
		m.runningWG.Wait()
		m.serverLoopStop = nil
	}
}

func (m *Multiplexer) Run() {
	log.Println("Starting VNC multiplexer...")

	// start initial server loop
	m.startProxyServerLoop()

	for {
		log.Println("Proxy client event loop started.")
		if err := m.proxyClient.RunEventLoop(1); err != nil {
			log.Printf("Proxy client event loop error: %v", err)
		}

		// Client connection lost here
		m.proxyClient.Close()

		if m.isConnected {
			m.isConnected = false
			if m.onConnectionOffline != nil {
				m.onConnectionOffline()
			}
		}

		// Keep proxyServer running; just show disconnected screen to currently
		// connected viewers (and future viewers) so they can still connect.
		if m.proxyServer != nil {
			m.drawDisconnectedScreen()
		}

		// Re-establish connection to the target server (proxyClient)
		for {
			log.Println("Attempting to reconnect to target server...")
			if err := m.initProxyClient(m.clientFactory); err == nil {
				break
			}
			time.Sleep(5 * time.Second)
		}

		// If framebuffer size changed, we need a new server; detect and recreate
		if m.proxyServer != nil {
			if m.proxyServer.GetWidth() != m.proxyClient.GetFrameBufferWidth() || m.proxyServer.GetHeight() != m.proxyClient.GetFrameBufferHeight() {
				log.Println("Framebuffer size changed, recreating proxy server.")

				// safely stop old server loop and close screen
				m.stopProxyServerLoop()
				m.proxyServer.Close()

				if err := m.initProxyServer(m.serverFactory); err != nil {
					log.Printf("Failed to recreate proxy server: %v", err)
					// fall back to continuing with old server (though closed) - try next loop
					continue
				}

				m.startProxyServerLoop()
			}
		} else {
			// if server was nil for some reason (first startup), create one
			if err := m.initProxyServer(m.serverFactory); err != nil {
				log.Printf("Failed to recreate proxy server: %v", err)
			} else {
				m.startProxyServerLoop()
			}
		}

		// reset handlers for new client or server
		m.setupHandlers()

		// ensure server event loop is running (idempotent)
		m.startProxyServerLoop()
	}
}

func (m *Multiplexer) SetConnectionOnlineCallback(callback func()) {
	m.onConnectionOnline = callback
}

func (m *Multiplexer) SetConnectionOfflineCallback(callback func()) {
	m.onConnectionOffline = callback
}

func (m *Multiplexer) RefreshVnc() {
	log.Println("RefreshVnc requested - recreating target connection")
	if m.proxyClient != nil {
		m.proxyClient.Close()
	}

	if m.isConnected {
		m.isConnected = false
		if m.onConnectionOffline != nil {
			m.onConnectionOffline()
		}
	}
}

func (m *Multiplexer) Close() {
	log.Println("Closing multiplexer...")

	// stop server loop first to avoid use-after-free
	m.stopProxyServerLoop()

	if m.proxyClient != nil {
		m.proxyClient.Close()
	}

	if m.proxyServer != nil {
		m.proxyServer.Close()
		m.proxyServer = nil
	}
}
