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
	client.SetPassword("password")

	// Configure app data (compression, quality, encodings, remote cursor)
	appConfig := vnc.AppDataConfig{
		CompressLevel:   2,
		QualityLevel:    6,
		Encodings:       "tight zrle ultra copyrect hextile zlib corre rre raw",
		UseRemoteCursor: true,
	}
	client.SetAppData(appConfig)

	client.SetStandardPixelFormat()

	client.SetCanHandleNewFBSize(false)

	var lastGotFrameBufferUpdate time.Time
	client.SetGotFrameBufferUpdateHandler(func(x, y, w, h int) {
		buffer := client.GetFrameBuffer()
		if buffer != nil {
			if time.Since(lastGotFrameBufferUpdate) >= 10*time.Second {
				fmt.Printf("Frame buffer data size: %d bytes\n", len(buffer))
				lastGotFrameBufferUpdate = time.Now()
			}
		}
	})

	var lastFinishedFrameBufferUpdate time.Time
	client.SetFinishedFrameBufferUpdateHandler(func() {
		if time.Since(lastFinishedFrameBufferUpdate) >= 10*time.Second {
			fmt.Println("Frame buffer update finished")
			lastFinishedFrameBufferUpdate = time.Now()
		}
	})

	if !client.Init() {
		log.Fatal("Failed to initialize VNC client")
	}

	fmt.Println("VNC client initialized successfully")
	fmt.Printf("Frame buffer size: %dx%d\n", client.GetFrameBufferWidth(), client.GetFrameBufferHeight())

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
