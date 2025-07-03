package main

import (
	"fmt"
	"libvnc-go/pkg/vnc"
	"log"
	"time"
)

func main() {
	client := vnc.NewClient(8, 3, 4)
	if client == nil {
		log.Fatal("Failed to create VNC client")
	}
	defer client.Close()

	client.SetHost("127.0.0.1")
	client.SetPort(5900)
	client.SetPassword("asdf")

	// Configure app data (compression, quality, encodings, remote cursor)
	client.SetAppData(2, 6, "tight zrle ultra copyrect hextile zlib corre rre raw", true)

	// Configure pixel format for standard (non-IPEPS) connections
	client.SetStandardPixelFormat()

	// Or for IPEPS connections, use:
	// client.SetIPEPSPixelFormat()

	// Set framebuffer size handling
	client.SetCanHandleNewFBSize(false)

	// Set up frame buffer update callback
	client.SetGotFrameBufferUpdateHandler(func(x, y, w, h int) {
		// Get framebuffer data
		buffer := client.GetFrameBuffer()
		if buffer != nil {
			fmt.Printf("Frame buffer data size: %d bytes\n", len(buffer))
		}
	})

	client.SetFinishedFrameBufferUpdateHandler(func() {
		fmt.Println("Frame buffer update finished")
	})

	if !client.Init() {
		log.Fatal("Failed to initialize VNC client")
	}

	fmt.Println("VNC client initialized successfully")

	client.SendFrameBufferUpdateRequest(0, 0, client.GetFrameBufferWidth(), client.GetFrameBufferHeight(), false)

	go func() {
		for {
			client.SendPointerEvent(100, 100, 0)
			time.Sleep(1 * time.Second)
			client.SendPointerEvent(200, 200, 0)
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for {
			client.SendKeyEvent(0xFFEB, true)
			client.SendKeyEvent(0xFFEB, false)
			time.Sleep(1 * time.Second)
		}
	}()

	fmt.Println("Running event loop...")
	err := client.RunEventLoop(10 * 1000 * 1000)
	if err != nil {
		log.Printf("Event loop error: %v", err)
	}

	fmt.Println("VNC client example completed")
}
