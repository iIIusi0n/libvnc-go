//go:build !cgo
// +build !cgo

package vnc

func init() {
	defaultClientFactory = func(bitsPerSample, samplesPerPixel, bytesPerPixel int) (ClientPort, error) {
		return nil, ErrCreateClient
	}
	defaultServerFactory = func(width, height, bitsPerSample, samplesPerPixel, bytesPerPixel int) (ServerPort, error) {
		return nil, ErrCreateServer
	}
}
