/**
 * PiFX implementation in Go
 */
package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"net"
	"time"

	pifx "github.com/andrewmunsell/pifx-go-lib"
	animations "github.com/andrewmunsell/pifx-go-lib/animations"
	pifxregister "github.com/andrewmunsell/pifx-go-lib/gob"
)

var off bool
var tcp bool
var tcp_port int
var http bool
var http_port int
var raw bool
var pixels int
var spi_device string

func main() {

	// Initialize the app and parse the command line flags
	flag.BoolVar(&off, "off", false, "Turns all LED pixels off")
	flag.BoolVar(&tcp, "tcp", false, "Listen on a TCP port for remote commands")
	flag.IntVar(&tcp_port, "port", 9123, "Port to bind a TCP server on to listen for remote commands")

	flag.BoolVar(&http, "http", false, "Enable or disable the HTTP server and interface")
	flag.IntVar(&http_port, "http_port", 8080, "Port to bind a TCP server on to listen for remote commands")

	flag.BoolVar(&raw, "raw", false, "Accept raw LED input over TCP. Also requires TCP to be enabled.")
	flag.IntVar(&pixels, "pixels", 25, "Number of LED pixels in the strand")
	flag.StringVar(&spi_device, "spi", "/dev/spidev0.0", "SPI device to use")

	flag.Parse()

	// Register gob types
	pifxregister.RegisterGobTypes()

	// Setup the pixel strand and animations
	strand := pifx.NewStrand(pixels, spi_device)
	anims := make([]animations.Animation, 0)

	// Setup the command channel. Anything that wants to change a pixel
	// simply writes to the channel and the main Goroutine will automatically
	// set the pixel and handle the timing.
	commandChannel := make(chan *pifx.PixelCommand)
	rawChannel := make(chan []byte)

	// Perform any "one off" actions that are specified
	// on the command line.
	if off {
		strand.Wipe(pifx.Pixel{0, 0, 0})
		strand.Write()

		return
	}

	// Let the user know something's happening
	fmt.Println("PiFX starting up...")

	if tcp && tcp_port > 0 {
		tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", tcp_port))

		if err != nil {
			panic(err)
		}

		t, err := net.ListenTCP("tcp", tcpAddr)

		if err != nil {
			panic(err)
		}

		if raw {
			go ListenTCPAcceptRaw(t, rawChannel)
		} else {
			go ListenTCPAccept(t, commandChannel)
		}
	}

	ticker := time.Tick(time.Second / 24)

	now := <-ticker

	// Run the main loop
	for {
		select {
		case cmd := <-commandChannel:
			anims = ProcessCommand(&strand, cmd, anims) // Process a queued command
		case buf := <-rawChannel:
			for i := 0; i < len(buf); i += 3 {
				strand.Set(i/3, pifx.Pixel{buf[i], buf[i+1], buf[i+2]})
			}
		case now = <-ticker:
			// Render the animations onto the strand
			if len(anims) > 0 {
				for _, a := range anims {
					(a).Render(now, &strand)
				}
			}

			strand.Write() // Write the current strand to SPI
		}
	}
}

func ProcessCommand(strand *pifx.Strand, cmd *pifx.PixelCommand, anims []animations.Animation) []animations.Animation {
	switch cmd.Action {
	case 0:
		payload, ok := cmd.Payload.([]*pifx.Pixel)

		if ok {
			for i, p := range payload {
				strand.Set(i, *p)
			}
		}
	case 1:
		strand.Wipe(pifx.Pixel{0, 0, 0})

		return make([]animations.Animation, 0)
	case 2:
		payload, ok := cmd.Payload.([]animations.Animation)

		if ok {
			newAnimations := make([]animations.Animation, len(anims)+len(payload))

			copy(newAnimations, anims)
			copy(newAnimations[len(anims):], payload)

			return newAnimations
		}
	}

	return anims
}

func ListenTCPAccept(t *net.TCPListener, commandChannel chan *pifx.PixelCommand) {
	for {
		conn, err := t.Accept()

		if err != nil {
			panic(err)
		}

		go ListenTCPConnection(conn, commandChannel)
	}
}

func ListenTCPConnection(t net.Conn, commandChannel chan *pifx.PixelCommand) {
	dec := gob.NewDecoder(t)

	defer t.Close()

	for {
		cmd := &pifx.PixelCommand{}

		err := dec.Decode(cmd)

		if err != nil {
			return
		}

		commandChannel <- cmd
	}
}

func ListenTCPAcceptRaw(t *net.TCPListener, rawChannel chan []byte) {
	for {
		conn, err := t.Accept()

		if err != nil {
			panic(err)
		}

		go ListenTCPConnectionRaw(conn, rawChannel)
	}
}

func ListenTCPConnectionRaw(t net.Conn, rawChannel chan []byte) {
	for {
		buf := make([]byte, pixels*3)

		n, err := t.Read(buf)

		if err != nil {
			return
		}

		if n == pixels*3 {
			rawChannel <- buf
		}
	}
}
