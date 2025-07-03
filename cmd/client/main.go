package main

import (
	"log"

	"libvnc-go/pkg/vnc"
)

func main() {
	client := vnc.NewClient(8, 3, 4)
	if client == nil {
		log.Println("Failed to create client")
		return
	}
	
	client.SetHost("127.0.0.1")
	client.SetPort(5900)
	client.SetPassword("asdf")
	
	client.SetGotFrameBufferUpdateHandler(func(x, y, w, h int) {
		log.Printf("Frame buffer updated: x=%d, y=%d, width=%d, height=%d\n", x, y, w, h)
	})
	
	if !client.Init() {
		log.Println("Failed to initialize client")
		return
	}

	if err := client.RunEventLoop(60 * 1000 * 1000); err != nil {
		log.Printf("Event loop error: %v", err)
	}
	
	client.Close()
}
