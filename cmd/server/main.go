package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"time"
	"unsafe"

	"github.com/libvnc-go/pkg/vnc"
)

func main() {
	server := vnc.NewServer(800, 600, 8, 3, 4)
	if server == nil {
		log.Fatal("Failed to create VNC server")
	}
	defer server.Close()

	server.SetPort(5900)
	server.SetPassword("password")

	server.SetStandardPixelFormat()

	server.SetKeyEventHandler(func(down bool, key uint32, clientPtr unsafe.Pointer) {
		action := "up"
		if down {
			action = "down"
		}
		fmt.Printf("Key event: key=0x%x (%d) %s\n", key, key, action)
	})

	server.SetPointerEventHandler(func(buttonMask, x, y int, clientPtr unsafe.Pointer) {
		fmt.Printf("Pointer event: x=%d, y=%d, buttons=%d\n", x, y, buttonMask)
	})

	server.SetNewClientHandler(func(clientPtr unsafe.Pointer) {
		fmt.Printf("New client connected. Total clients: %d\n", server.GetClientCount())
	})

	err := server.InitServer()
	if err != nil {
		log.Fatal("Failed to initialize VNC server:", err)
	}

	fmt.Printf("VNC server started on port 5900\n")
	fmt.Printf("Resolution: %dx%d\n", server.GetWidth(), server.GetHeight())
	fmt.Printf("Password: password\n")

	go testNewClientWithConn(server)

	go func() {
		frameBuffer := server.GetFrameBuffer()
		width := server.GetWidth()
		height := server.GetHeight()

		var frame uint64 = 0
		for {
			for y := range height {
				for x := range width {
					waveX := math.Sin(float64(x)*0.02 + float64(frame)*0.1)
					waveY := math.Sin(float64(y)*0.02 + float64(frame)*0.1)
					wave := waveX + waveY

					intensity := uint8((wave + 2) * 63.5)

					offset := (y*width + x) * 4

					frameBuffer[offset] = intensity
					frameBuffer[offset+1] = intensity / 2
					frameBuffer[offset+2] = intensity / 4
					frameBuffer[offset+3] = 0
				}
			}

			server.MarkRectAsModified(0, 0, width, height)

			frame++
			time.Sleep(50 * time.Millisecond)
		}
	}()

	fmt.Println("Server is running. Press Ctrl+C to stop.")

	for {
		server.ProcessEvents(100)

		if !server.IsActive() {
			fmt.Println("Server is no longer active")
			break
		}
	}

	fmt.Println("VNC server stopped")
}

func testNewClientWithConn(server *vnc.Server) {
	fmt.Println("\n=== Testing NewClientWithConn ===")

	time.Sleep(500 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:5900")
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("Successfully connected to server: %s -> %s\n", conn.LocalAddr(), conn.RemoteAddr())

	client, err := vnc.NewClientWithConn(server, conn)
	if err != nil {
		fmt.Printf("Failed to create client with connection: %v\n", err)
		return
	}

	fmt.Printf("Successfully created RFB client with connection: %p\n", client.GetPointer())

	if client.GetPointer() == nil {
		fmt.Println("Client pointer is nil")
		return
	}

	fmt.Println("Keeping connection alive for 3 seconds...")
	time.Sleep(3 * time.Second)

	fmt.Println("Test completed successfully!")
}
