//go:build cgo
// +build cgo

package vnc

func init() {
	defaultClientFactory = func(bitsPerSample, samplesPerPixel, bytesPerPixel int) (ClientPort, error) {
		c := NewClient(bitsPerSample, samplesPerPixel, bytesPerPixel)
		if c == nil {
			return nil, ErrCreateClient
		}
		return c, nil
	}

	defaultServerFactory = func(width, height, bitsPerSample, samplesPerPixel, bytesPerPixel int) (ServerPort, error) {
		s := NewServer(width, height, bitsPerSample, samplesPerPixel, bytesPerPixel)
		if s == nil {
			return nil, ErrCreateServer
		}
		return s, nil
	}
}
