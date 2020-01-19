package main

import "github.com/splace/joysticks"
import "log"
import "time"
import "math"
import "go.bug.st/serial.v1"
import "encoding/hex"

func main() {
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity: serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open("/dev/ttyUSB0", mode)
	if err != nil {
		log.Fatal(err)
	}

	var pan, oldpan int8 = 0,0
	var tilt, oldtilt int8 = 0,0
	var zoom, oldzoom int8 = 0,0
	var focus, oldfocus int8 = 0,0
	// try connecting to specific controller.
	// the index is system assigned, typically it increments on each new controller added.
	// indexes remain fixed for a given controller, if/when other controller(s) are removed.
	device := joysticks.Connect(1)

	if device == nil {
		panic("no HIDs")
	}

	// using Connect allows a device to be interrogated
	log.Printf("HID#1:- Buttons:%d, Hats:%d\n", len(device.Buttons), len(device.HatAxes)/2)

	// get/assign channels for specific events
	b1press := device.OnClose(1)
	b2press := device.OnClose(2)
	b3press := device.OnClose(3)
	b4press := device.OnClose(4)
	b5press := device.OnClose(5)
	b6press := device.OnClose(6)
	b7press := device.OnClose(7)
	b8press := device.OnClose(8)
	b9press := device.OnClose(9)
	b10press := device.OnClose(10)
	b11press := device.OnClose(11)
	h1move := device.OnMove(1)
	h2move := device.OnMove(2)
	h3move := device.OnMove(3)
	h4move := device.OnMove(4)
        jevent := device.OSEvents

	// start feeding OS events onto the event channels.
	go device.ParcelOutEvents()

	// handle event channels
	go func(){
		for{
			select {
                        case oe := <-jevent:
                                if((0==oe.Index) && (0==oe.Type) && (0==oe.Value)) {
                                        panic("null events")
                                }
			case h1 := <-h1move:
				hpos:=h1.(joysticks.CoordsEvent)
				if(pan != int8(math.Floor(float64(22*hpos.X)))) {
					pan = int8(math.Floor(float64(22*hpos.X)))
				}
				if(tilt != int8(math.Floor(float64(-20*hpos.Y)))) {
					tilt = int8(math.Floor(float64(-20*hpos.Y)))
				}
			case h2 := <-h2move:
				hpos:=h2.(joysticks.CoordsEvent)
//				if(pan != int8(math.Floor(float64(7*hpos.X)))) {
//					pan = int8(math.Floor(float64(7*hpos.X)))
//				}
				if(focus != int8(math.Floor(float64(10*hpos.Y)))) {
					focus = int8(math.Floor(float64(10*hpos.Y)))
				}
			case h3 := <-h3move:
				hpos:=h3.(joysticks.CoordsEvent)
				if(zoom != int8(math.Floor(float64(-7*hpos.X)))) {
					zoom = int8(math.Floor(float64(-7*hpos.X)))
				}
//				if(tilt != int8(math.Floor(float64(7*hpos.Y)))) {
//					tilt = int8(math.Floor(float64(7*hpos.Y)))
//				}
			case h4 := <-h4move:
				hpos:=h4.(joysticks.CoordsEvent)
				log.Println("hat #4 moved to:", hpos.X,hpos.Y)
			case <-b1press:
				log.Println("button #1 pressed")
			case <-b2press:
				log.Println("button #2 pressed")
			case <-b3press:
				log.Println("button #3 pressed")
			case <-b4press:
				log.Println("button #4 pressed")
			case <-b5press:
				log.Println("button #5 pressed")
			case <-b6press:
				log.Println("button #6 pressed")
			case <-b7press:
				log.Println("button #7 pressed")
			case <-b8press:
				log.Println("button #8 pressed")
			case <-b9press:
				log.Println("button #9 pressed")
			case <-b10press:
				log.Println("button #10 pressed")
			case <-b11press:
				log.Println("button #11 pressed")
			}
		}
	}()
	for {
		// take care with these shared variables!
		// they are single-byte to avoid race issues
		// only write them in the joystick routine
		// read them here and watch for changes
		time.Sleep(time.Millisecond*125)
		if(oldpan != pan) {
			oldpan = pan
			log.Println("Pan is now:", oldpan)
			sendPanTilt(port, 8, pan, tilt) // 8 is broadcast to all cameras
		}
		if(oldtilt != tilt) {
			oldtilt = tilt
			log.Println("Tilt is now:", oldtilt)
			sendPanTilt(port, 8, pan, tilt) // 8 is broadcast to all cameras
		}
		if(oldzoom != zoom) {
			oldzoom = zoom
			log.Println("Zoom is now:", oldzoom)
			sendZoom(port, 8, zoom) // 8 is broadcast to all cameras
		}
		if(oldfocus != focus) {
			oldfocus = focus
			log.Println("Focus is now:", oldfocus)
			sendFocus(port, 8, focus) // 8 is broadcast to all cameras
		}
	}

	log.Println("Timeout in 10 secs.")
	time.Sleep(time.Second*10)
	log.Println("Shutting down due to timeout.")
}

func sendZoom(port serial.Port, cam byte, zoom int8) {
	if((zoom>0) && (zoom<=7)) {
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x04, 0x07, (0x20+(byte(zoom))), 0xFF})
	} else if((zoom<0) && (zoom>=-7)) {
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x04, 0x07, (0x30+(byte(0-zoom))), 0xFF})
	} else {
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x04, 0x07, 0x00, 0xFF})
	}
}

func sendFocus(port serial.Port, cam byte, focus int8) {
	if((focus>0) && (focus<=7)) {
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x02, 0xFF})
	} else if((focus<0) && (focus>=-7)) {
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x03, 0xFF})
	} else {
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x00, 0xFF})
	}
}

func sendPanTilt(port serial.Port, cam byte, pan int8, tilt int8) {
	if(pan>22) {pan = 0}
	if(pan<(-22)) {pan = 0}
	if(tilt>20) {tilt = 0}
	if(tilt<(-20)) {tilt = 0}
	if((pan==0) && (tilt==0)) { // Stop
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, 0x00, 0x00, 0x03, 0x03, 0xFF})
	} else if((pan==0) && (tilt>0)) { // Up
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, 0x00, byte(tilt), 0x03, 0x01, 0xFF})
	} else if((pan==0) && (tilt<0)) { // Down
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, 0x00, byte(0-tilt), 0x03, 0x02, 0xFF})
	} else if((pan<0) && (tilt==0)) { // Left
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(0-pan), 0x00, 0x01, 0x03, 0xFF})
	} else if((pan>0) && (tilt==0)) { // Right
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(pan), 0x00, 0x02, 0x03, 0xFF})
	} else if((pan<0) && (tilt>0)) { // UpLeft
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(0-pan), byte(tilt), 0x01, 0x01, 0xFF})
	} else if((pan>0) && (tilt>0)) { // UpRight
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(pan), byte(tilt), 0x02, 0x01, 0xFF})
	} else if((pan<0) && (tilt<0)) { // DownLeft
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(0-pan), byte(0-tilt), 0x01, 0x02, 0xFF})
	} else if((pan>0) && (tilt<0)) { // DownRight
		sendVisca(port, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(pan), byte(0-tilt), 0x02, 0x02, 0xFF})
	}
}

func sendVisca(port serial.Port, message []byte) {
	n, err := port.Write(message)
	log.Println(hex.Dump(message))
	_ = n
	if err != nil {
		log.Fatal(err)
	}
}
