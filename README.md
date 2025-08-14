# libvnc-go

[![Go Reference](https://pkg.go.dev/badge/github.com/iIIusi0n/libvnc-go.svg)](https://pkg.go.dev/github.com/iIIusi0n/libvnc-go)

A Go wrapper of libvncserver/libvncclient providing VNC client and server functionality.

## Overview

libvnc-go is a Go library that wraps the native libvncserver and libvncclient C libraries, providing a clean and idiomatic Go interface for VNC operations. It supports both VNC client and server functionality.

## Features

### VNC Client
- Connect to VNC servers
- Handle framebuffer updates
- Send keyboard and mouse events
- Configurable pixel formats
- Password authentication

### VNC Server
- Create VNC servers
- Handle client connections
- Process keyboard and mouse input
- Update framebuffer content
- Password protection

## Installation

### Prerequisites

You need to have libvncserver and libvncclient development libraries installed on your system.

#### Windows:

Recommended to use MSYS2 which has the necessary packages.

### Go Installation

```bash
go get github.com/iIIusi0n/libvnc-go
```

## Quick Start

Check out the examples in the `cmd` directory.

## Dependencies

- Go 1.22.2 or later
- libvncserver development libraries
- libvncclient development libraries

## Disclaimer

This project is incomplete and not ready for production use.
