package main

import "github.com/splace/joysticks"
import "log"
import "strings"
import "bufio"
import "time"
import "math"
import "fmt"
//import "go.bug.st/serial.v1"
//import "go.bug.st/serial.v1/enumerator"
import "encoding/hex"
import "os/signal"
import "syscall"
import "os"
import "net"

var netAddr = "10.2.1.146" // "192.168.110.110"
var netPort = "1259" // "52381"
var killSignal chan os.Signal

// must install:
// go get -u golang.org/x/tools/cmd/stringer
// then run:
// stringer -type=WhiteBalanceT

// WhiteBalanceT Type definition for an enum of White Balance camera settings
type WhiteBalanceT int

// Type values
const (
	wbUndefined WhiteBalanceT = iota // iota auto-increments
	wbAuto
	wbIndoor
	wbOutdoor
	wbOnePush
	wbManual
	wbOutdoorAuto
	wbSodiumLampAuto
	wbSodiumAuto
)

var loop1, loop2, loop3, loop4 uint8 = 0,0,0,0
var slowPT, oldSlowPT, slowZ, oldSlowZ bool = false, false, false, false

/*
var TestState TestStateT

func nullState() error {
        println(TestState.String())
        return nil
}
*/

func main() {
	killSignal = make(chan os.Signal, 1)
	cameraErrChan := make(chan bool)
	cameraSendChan := make(chan []byte)
	cameraReceiveChan := make(chan []byte)
	controllerDisconnectChan := make(chan bool)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGSTOP, syscall.SIGQUIT)

	go cameraComm(cameraSendChan, cameraReceiveChan, cameraErrChan)

	var pan, oldpan int8 = 0,0
	var tilt, oldtilt int8 = 0,0
	var zoom, oldzoom int8 = 0,0
	var focus, oldfocus int8 = 0,0
	// try connecting to specific controller.
	// the index is system assigned, typically it increments on each new controller added.
	// indexes remain fixed for a given controller, if/when other controller(s) are removed.
	device := joysticks.Connect(1)

	if device == nil {
		panic("no HID Joystick/Controllers detected")
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
	b1release := device.OnOpen(1)
	b2release := device.OnOpen(2)
	b3release := device.OnOpen(3)
	b4release := device.OnOpen(4)
	b5release := device.OnOpen(5)
	b6release := device.OnOpen(6)
	b7release := device.OnOpen(7)
	b8release := device.OnOpen(8)
	b9release := device.OnOpen(9)
	b10release := device.OnOpen(10)
	b11release := device.OnOpen(11)
	h1move := device.OnMove(1)
	h2move := device.OnMove(2)
	h3move := device.OnMove(3)
	h4move := device.OnMove(4)
        jevent := device.OSEvents

	// start feeding OS events onto the event channels.
	go device.ParcelOutEvents()

	// handle event channels
	go func() {
		for {
			loop1 = loop1 + 1
			select {
                        case oe := <-jevent:
                                if((0==oe.Index) && (0==oe.Type) && (0==oe.Value)) {
                                        panic("null events")
					controllerDisconnectChan<-true
                                }
			case h1 := <-h1move:
				hpos:=h1.(joysticks.CoordsEvent)
				//log.Println("Pos: ", hpos.X, "x, ", hpos.Y, "y")
				if(pan != int8(math.Floor(float64(24*hpos.X)))) {
					pan = int8(math.Floor(float64(24*hpos.X)))
				}
				if(tilt != int8(math.Floor(float64(-20*hpos.Y)))) {
					tilt = int8(math.Floor(float64(-20*hpos.Y)))
				}
			case h2 := <-h2move:
				hpos:=h2.(joysticks.CoordsEvent)
				if(focus != int8(math.Floor(float64(10*hpos.Y)))) {
					focus = int8(math.Floor(float64(10*hpos.Y)))
				}
			case h3 := <-h3move:
				hpos:=h3.(joysticks.CoordsEvent)
				if(zoom != int8(math.Floor(float64(-7*hpos.X)))) {
					zoom = int8(math.Floor(float64(-7*hpos.X)))
				}
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
				log.Println("button #5 pressed STOPPING PAN/TILT")
				tilt = 0
				oldtilt = 0
				pan = 0
				oldpan = 0
				sendPanTilt(cameraSendChan, 8, pan, tilt) // 8 is broadcast to all cameras
				slowPT = false
				slowZ = false
			case <-b6press:
				log.Println("button #6 pressed STOPPING ZOOM/FOCUS")
				zoom = 0
				oldzoom = 0
				sendZoom(cameraSendChan, 8, zoom) // 8 is broadcast to all cameras
				focus = 0
				oldfocus = 0
				sendFocus(cameraSendChan, 8, focus) // 8 is broadcast to all cameras
				slowPT = false
				slowZ = false
			case <-b7press:
				log.Println("button #7 pressed")
			case <-b8press:
				log.Println("button #8 pressed")
			case <-b9press:
				log.Println("button #9 pressed")
			case <-b10press:
				log.Println("button #10 pressed SLOW PAN/TILT")
				slowPT = true
			case <-b11press:
				slowZ = true
				log.Println("button #11 pressed SLOW ZOOM")
			case <-b1release:
				log.Println("button #1 released")
			case <-b2release:
				log.Println("button #2 released")
			case <-b3release:
				log.Println("button #3 released")
			case <-b4release:
				log.Println("button #4 released")
			case <-b5release:
				log.Println("button #5 released STOPPING PAN/TILT")
				tilt = 0
				oldtilt = 0
				pan = 0
				oldpan = 0
				sendPanTilt(cameraSendChan, 8, pan, tilt) // 8 is broadcast to all cameras
				slowPT = false
				slowZ = false
			case <-b6release:
				log.Println("button #6 released STOPPING ZOOM/FOCUS")
				zoom = 0
				oldzoom = 0
				sendZoom(cameraSendChan, 8, zoom) // 8 is broadcast to all cameras
				focus = 0
				oldfocus = 0
				sendFocus(cameraSendChan, 8, focus) // 8 is broadcast to all cameras
				slowPT = false
				slowZ = false
			case <-b7release:
				log.Println("button #7 released")
			case <-b8release:
				log.Println("button #8 released")
			case <-b9release:
				log.Println("button #9 released")
			case <-b10release:
				log.Println("button #10 released FAST PAN/TILT")
				slowPT = false
			case <-b11release:
				log.Println("button #11 released FAST ZOOM")
				slowZ = false
			}
		}
		log.Println("exiting event capture goroutine")
	}()

	go func() {
		for {
			loop2 = loop2 + 1
			// take care with these shared variables!
			// they are single-byte to avoid race issues
			// only write them in the joystick routine
			// read them here and watch for changes
			//log.Println("loop ", loop1, " ", loop2 , " ", loop3, " ", loop4)
			time.Sleep(time.Millisecond*125)
			if(oldpan != pan) {
				oldpan = pan
				log.Println("Pan is now:", oldpan)
				sendPanTilt(cameraSendChan, 8, speedLimit(pan, slowPT), speedLimit(tilt, slowPT)) // 8 is broadcast to all cameras
			}
			if(oldtilt != tilt) {
				oldtilt = tilt
				log.Println("Tilt is now:", oldtilt)
				sendPanTilt(cameraSendChan, 8, speedLimit(pan, slowPT), speedLimit(tilt, slowPT)) // 8 is broadcast to all cameras
			}
			if(oldSlowPT != slowPT) {
				oldSlowPT = slowPT
				log.Println("Tilt speed change")
				sendPanTilt(cameraSendChan, 8, speedLimit(pan, slowPT), speedLimit(tilt, slowPT)) // 8 is broadcast to all cameras
			}
			if((oldzoom != zoom) || (oldSlowZ != slowZ)) {
				oldzoom = zoom
				if(slowZ) {
					log.Println("Zooming SLOWLY")
					if(zoom>0) {
						sendZoom(cameraSendChan, 8, 1) // 8 is broadcast to all cameras
					} else if(zoom<0) {
						sendZoom(cameraSendChan, 8, -1) // 8 is broadcast to all cameras
					} else {
						sendZoom(cameraSendChan, 8, 0) // 8 is broadcast to all cameras
					}
				} else {
					log.Println("Zoom is now:", oldzoom)
					sendZoom(cameraSendChan, 8, zoom) // 8 is broadcast to all cameras
				}
			}
			if(oldfocus != focus) {
				oldfocus = focus
				log.Println("Focus is now:", oldfocus)
				sendFocus(cameraSendChan, 8, focus) // 8 is broadcast to all cameras
			}
		}
		log.Println("exiting final for loop")
	}()
	select {
		case <-killSignal:
			log.Println("got kill signal!")
		case <-cameraErrChan:
			log.Println("camera communication error!")
		case <-controllerDisconnectChan:
			log.Println("USB joystick disconnect error!")
	}

/*	gotoCloseShot()
	time.Sleep(100 * time.Millisecond) */
	log.Println("exiting!")
}
/*
func gotoWideShot () {
	gotoZoom(cameraSendChan, 8, 1000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 980, 65536 - 180)
}

func gotoCloseShot () {
	gotoZoom(cameraSendChan, 8, 12500)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 982, 65536 - 123)
}

func gotoLeftShot () {
	gotoZoom(cameraSendChan, 8, 8000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 1100, 65536 - 123)
}

func gotoTempShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, 8, 14500)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536-97, 65536-132)
}

func gotoRightShot () {
	gotoZoom(cameraSendChan, 8, 8000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 860, 65536 - 123)
}

func gotoPianoShot () {
	gotoZoom(cameraSendChan, 8, 12000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 1302, 65536 - 100)
}

func gotoDirectorShot () {
	gotoZoom(cameraSendChan, 8, 12000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 950, 65536 - 90)
}

func gotoOrganShot () {
	gotoZoom(cameraSendChan, 8, 12000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 915, 65536 - 100)
}

func gotoCloseLeftShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, 8, 13000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 26, 65536 - 95)
}

func gotoCloseRightShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, 8, 13000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 14, 65536 - 95)
}

func gotoMediumShot () {
	gotoZoom(cameraSendChan, 8, 5000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 980, 65536 - 127)
}

func gotoMediumCloseShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, 8, 10500)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 6, 65536 - 95)
}

func gotoChoirShot () {
	gotoZoom(cameraSendChan, 8, 7000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536 - 1060, 65536 - 100)
}

func gotoWideScreenShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, 8, 2000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536-3, 65536-80)
}

func gotoScreenShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, 8, 11000)
	gotoPanTilt(cameraSendChan, 8, 10, 10, 65536-3, 20)
}
*/
func cameraComm(cameraSendChan <-chan []byte, cameraReceiveChan chan<- []byte, cameraErrChan chan<- bool) {
//	udpbuf := make([]byte, 2048)
	udpconn, err := net.Dial("udp", netAddr+":"+netPort)
	if err != nil {
		fmt.Printf("Got an error opening the UDP port %v", err)
		cameraErrChan <- true
	}
	defer udpconn.Close()
	// cameraSendChan are bytes sent TO the camera over the network or port
	// cameraReceiveChan are bytes received back from the camera
	var camReader *bufio.Reader
	var camScanner *bufio.Scanner
	camReader = bufio.NewReader(udpconn)
	camScanner = bufio.NewScanner(camReader)
	// Visca messages end in 0xFF, so use that as the termination character
	// for reading responses back from the serial port (the 0xFF will be stripped)
	camScanner.Split(AnySplit("\xFF"))
	go cameraRead(camScanner, cameraReceiveChan, cameraErrChan)
	for (true) {
		select {
		case txmsg := <-cameraSendChan:
			_, err := fmt.Fprintf(udpconn, "%s", txmsg) // _ throws away byte count written from Fprintf
			if nil != err {
				log.Println("error when sending message: " + err.Error())
			} else {
				log.Println("message sent: " + string(txmsg))
			}
		}
	}
}

func cameraRead(scanner *bufio.Scanner, cameraReceiveChan chan<- []byte, cameraErrChan chan<- bool) {
	run := true
	for (run) {
		loop3 = loop3 + 1
		scanner.Scan()
		readbytes := []byte(scanner.Text())
		log.Println("Camera Response: ", hex.Dump(readbytes))
		cameraReceiveChan <- readbytes
		scanerror := scanner.Err()
		if (nil != scanerror) {
			run = false
			log.Print(scanerror)
		}
	}
	log.Println("exiting serial read goroutine")
	cameraErrChan<-true
}

/*func cameraWrite() {
	return
}*/

func sendZoom(cameraSendChan chan<- []byte, cam byte, zoom int8) {
	if((zoom>0) && (zoom<=7)) {
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x07, (0x20+(byte(zoom))), 0xFF})
	} else if((zoom<0) && (zoom>=-7)) {
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x07, (0x30+(byte(0-zoom))), 0xFF})
	} else {
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x07, 0x00, 0xFF})
	}
}

func gotoZoom(cameraSendChan chan<- []byte, cam byte, zoom int16) {
	// Direct zoom level command from 0x0 (wide) to 0x4000 (telephoto)
	if((zoom>=0) && (zoom<=0x4000)) {
		p := byte(0x0F & (zoom >> 12))
		q := byte(0x0F & (zoom >> 8))
		r := byte(0x0F & (zoom >> 4))
		s := byte(0x0F & zoom)
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x47, p, q, r, s, 0xFF})
	}
}

func stopZoom(cameraSendChan chan<- []byte, cam byte) {
	sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x07, 0x00, 0xFF})
}

func gotoFocus(cameraSendChan chan<- []byte, cam byte, focus int16) {
	// Direct focus level command, levels may not be specified, using the same as zoom
	if((focus>=0) && (focus<=0x4000)) {
		p := byte(0x0F & (focus >> 12))
		q := byte(0x0F & (focus >> 8))
		r := byte(0x0F & (focus >> 4))
		s := byte(0x0F & focus)
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x48, p, q, r, s, 0xFF})
	}
}

func stopFocus(cameraSendChan chan<- []byte, cam byte) {
	sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x00, 0xFF})
}

func onePushAutoFocus(cameraSendChan chan<- []byte, cam byte) {
	sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x18, 0x01, 0xFF})
}

func sendFocus(cameraSendChan chan<- []byte, cam byte, focus int8) {
	if(focus>0) {
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x02, 0xFF})
	} else if(focus<0) {
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x03, 0xFF})
	} else {
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x08, 0x00, 0xFF})
	}
}

func gotoZoomFocus(cameraSendChan chan<- []byte, cam byte, zoom int16, focus int16) {
	// Direct zoom level command from 0x0 (wide) to 0x4000 (telephoto)
	if((zoom>=0) && (zoom<=0x4000) && (focus>=0) && (focus<=0x4000)) {
		p := 0x0F & byte(zoom >> 12)
		q := 0x0F & byte(zoom >> 8)
		r := 0x0F & byte(zoom >> 4)
		s := 0x0F & byte(zoom)
		t := 0x0F & byte(focus >> 12)
		u := 0x0F & byte(focus >> 8)
		v := 0x0F & byte(focus >> 4)
		w := 0x0F & byte(focus)
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x47, p, q, r, s, t, u, v, w, 0xFF})
	}
}

func gotoPanTilt(cameraSendChan chan<- []byte, cam byte, panspeed int16, tiltspeed int16, pan uint16, tilt uint16) {
	// Direct pan and tilt command at specific speed
	var m, n byte
	if(panspeed>24) {panspeed = 0}
	if(panspeed<(-24)) {panspeed = 0}
	if(panspeed>=0) {m=byte(panspeed)} else {m=byte(0-panspeed)}
	if(tiltspeed>20) {tiltspeed = 0}
	if(tiltspeed<(-20)) {tiltspeed = 0}
	if(tiltspeed>=0) {n=byte(tiltspeed)} else {n=byte(0-tiltspeed)}
	p := 0x0F & byte(pan >> 12)
	q := 0x0F & byte(pan >> 8)
	r := 0x0F & byte(pan >> 4)
	s := 0x0F & byte(pan)
	t := 0x0F & byte(tilt >> 12)
	u := 0x0F & byte(tilt >> 8)
	v := 0x0F & byte(tilt >> 4)
	w := 0x0F & byte(tilt)
	sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x02, m, n, p, q, r, s, t, u, v, w, 0xFF})
}

func sendPanTilt(cameraSendChan chan<- []byte, cam byte, pan int8, tilt int8) {
	if(pan>24) {pan = 0}
	if(pan<(-24)) {pan = 0}
	if(tilt>20) {tilt = 0}
	if(tilt<(-20)) {tilt = 0}
	if((pan==0) && (tilt==0)) { // Stop
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, 0x00, 0x00, 0x03, 0x03, 0xFF})
	} else if((pan==0) && (tilt>0)) { // Up
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, 0x00, byte(tilt), 0x03, 0x01, 0xFF})
	} else if((pan==0) && (tilt<0)) { // Down
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, 0x00, byte(0-tilt), 0x03, 0x02, 0xFF})
	} else if((pan<0) && (tilt==0)) { // Left
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(0-pan), 0x00, 0x01, 0x03, 0xFF})
	} else if((pan>0) && (tilt==0)) { // Right
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(pan), 0x00, 0x02, 0x03, 0xFF})
	} else if((pan<0) && (tilt>0)) { // UpLeft
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(0-pan), byte(tilt), 0x01, 0x01, 0xFF})
	} else if((pan>0) && (tilt>0)) { // UpRight
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(pan), byte(tilt), 0x02, 0x01, 0xFF})
	} else if((pan<0) && (tilt<0)) { // DownLeft
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(0-pan), byte(0-tilt), 0x01, 0x02, 0xFF})
	} else if((pan>0) && (tilt<0)) { // DownRight
		sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x01, byte(pan), byte(0-tilt), 0x02, 0x02, 0xFF})
	}
}

func sendWhiteBalance(cameraSendChan chan<- []byte, cam byte, wbValue WhiteBalanceT) {
	switch wbValue {
		case wbAuto:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x00, 0xFF})
		case wbIndoor:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x01, 0xFF})
		case wbOutdoor:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x02, 0xFF})
		case wbOnePush:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x03, 0xFF})
		case wbManual:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x05, 0xFF})
		case wbOutdoorAuto:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x06, 0xFF})
		case wbSodiumLampAuto:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x07, 0xFF})
		case wbSodiumAuto:
			sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x04, 0x35, 0x08, 0xFF})
		default:
		// unknown white balance value
	}
}

func sendVisca(cameraSendChan chan<- []byte, message []byte) {
	cameraSendChan <- message
	log.Println(hex.Dump(message))
}

func AnySplit(substring string) func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && 0==len(data) {
			return 0, nil, nil
		}
		if i := strings.Index(string(data), substring); i >= 0 {
			return i + len(substring), data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return
	}
}

func speedLimit(speed int8, limited bool) (limitedspeed int8) {
	if(limited) {
		if(speed>0) {
			return 1
		} else if(speed<0) {
			return -1
		}
	return 0
	}
	return speed
}

// Read Pan Tilt Position
// 0x8x 0x09 0x06 0x12 0xFF (query)
// 0xy0 0x50 0x0p 0x0q 0x0r 0x0s 0x0t 0x0u 0x0v 0x0w 0xFF (response)
// 0xpqrs - pan position
// 0xtuvw - tilt position
