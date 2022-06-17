package main

import "github.com/splace/joysticks"
import "log"
import "strings"
import "strconv"
//import "bufio"
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
import "encoding/binary"
import "bytes"

var netAddr = "10.2.1.146" // "192.168.110.110"
var netPort = "1259" // "52381"
var camAddr byte = 8 // 8 is broadcast when on a serial link

var timeSlicesPerPanStep, panStepsPerTimeslice int16 = 0,0
var timeSlicesPerTiltStep, tiltStepsPerTimeslice int16 = 0,0
var targetPan, targetTilt int16 = 0,0
var startPan, startTilt int16 = 0,0
var endPan, endTilt int16 = 0,0
var startPTZF = PTZF{pan: -500, tilt: -250, zoom: 0}
var endPTZF = PTZF{pan: 500, tilt: 250, zoom: 4400}
var commandPTZF, oldTargetPTZF PTZF

var pan, oldpan int8 = 0,0
var tilt, oldtilt int8 = 0,0
var zoom, oldzoom int8 = 0,0
var focus, oldfocus int8 = 0,0
var killSignal = make(chan os.Signal, 1)
var cameraErrChan = make(chan bool)
var cameraSendChan = make(chan []byte)
var cameraReceiveChan = make(chan []byte)
var controllerDisconnectChan = make(chan bool)

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

type PTZF struct {
	pan int16
	tilt int16
	zoom int16
	focus int16
}

type stepPTZ struct {
	msPauseBefore uint32
	msMoveDuration uint32
	targetPan int16
	targetTilt int16
	targetZoom int16
	targetFocus int16 // -1 for no focus setting (auto)
	msPauseAfter uint32
}

type fixedPTZ struct {
	msPauseBefore uint32
	panSpeed int16
	tiltSpeed int16
	targetPan int16
	targetTilt int16
	targetZoom int16
	targetFocus int16 // -1 for no focus setting (auto)
	msPauseAfter uint32
}

var homePTZ = fixedPTZ{panSpeed: 1, tiltSpeed: 1}

func sendFixedPTZ(target fixedPTZ) {
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -980, -180)
}

func i16toBytes (anyi16 int16) ([]byte) {
	var endBytes = []byte{0, 0}
	binary.BigEndian.PutUint16(endBytes, uint16(anyi16))
	return endBytes
}

func bytesToi16 (thebytes []byte) (thei16 int16) {
	return int16(binary.BigEndian.Uint16(thebytes))
}

var loop1, loop2, loop3, loop4 uint8 = 0,0,0,0
var slowPT, oldSlowPT, slowZ, oldSlowZ bool = false, false, false, false
type timeTrigger struct {
	startTime time.Time
	duration time.Duration
	triggered bool
}
var mainTimer timeTrigger

func startTimer(timeDuration uint32) {
//	time.Sleep(2 * time.Second)
	mainTimer = timeTrigger{startTime: time.Now(),
		duration: time.Duration(timeDuration)*time.Millisecond,
		triggered: false}
}

func checkTimer() (msRemaining uint32, triggerNow bool) {
	if(mainTimer.triggered) {
		// timer already elapsed
		return 0, false
	} else {
		elapsedTime := time.Since(mainTimer.startTime)
		msElapsed := elapsedTime.Milliseconds()
		completionTarget := mainTimer.duration.Milliseconds()
		if (msElapsed < completionTarget) {
			return uint32((completionTarget - msElapsed)), false
		} else {
			mainTimer.triggered = true
			return 0, true
		}
	}
}

func checkTimerFraction() (fractionComplete float64, triggerNow bool) {
	if(mainTimer.triggered) {
		// timer already elapsed
		return 100.0, false
	} else {
		elapsedTime := time.Since(mainTimer.startTime)
		completionTarget := mainTimer.duration
		if (elapsedTime < completionTarget) {
			return float64(elapsedTime.Seconds() / completionTarget.Seconds()), false
		} else {
			mainTimer.triggered = true
			return 100.0, true
		}
	}
}

func interpolatePTZ(fractionComplete float64, startingPTZF PTZF, endingPTZF PTZF) (currentPTZF PTZF) {
	if fractionComplete > 1.0 { fractionComplete = 1.0 }
	if fractionComplete < 0.0 { fractionComplete = 0.0 }
	fractionRemaining := 1.0 - fractionComplete
	currentPTZF.pan = int16(fractionComplete * float64(endingPTZF.pan) + fractionRemaining * float64(startingPTZF.pan))
	currentPTZF.tilt = int16(fractionComplete * float64(endingPTZF.tilt) + fractionRemaining * float64(startingPTZF.tilt))
	currentPTZF.zoom = int16(fractionComplete * float64(endingPTZF.zoom) + fractionRemaining * float64(startingPTZF.zoom))
	currentPTZF.focus = int16(fractionComplete * float64(endingPTZF.focus) + fractionRemaining * float64(startingPTZF.focus))
	return 
}

func gotoPTZF(targetPTZF PTZF) {
	if 0 ==targetPTZF.focus {
		if(oldTargetPTZF.zoom != targetPTZF.zoom) {
			gotoZoom(cameraSendChan, camAddr, targetPTZF.zoom)
		}
	} else {
		if( (oldTargetPTZF.zoom != targetPTZF.zoom) || (oldTargetPTZF.focus != targetPTZF.focus) ) {
			gotoZoomFocus(cameraSendChan, camAddr, targetPTZF.zoom, targetPTZF.focus)
		}
	}
	if( (oldTargetPTZF.pan != targetPTZF.pan) || (oldTargetPTZF.tilt != targetPTZF.tilt) ) {
		gotoPanTilt(cameraSendChan, camAddr, 1, 1, targetPTZF.pan, targetPTZF.tilt) // panspeed, tiltspeed, pan, tilt
	}
	oldTargetPTZF = commandPTZF
}

func gotoPTZFspeed(targetPTZF PTZF, panspeed int16, tiltspeed int16) {
	// TODO: would like a variable speed zoom here
	if 0 ==targetPTZF.focus {
		if(oldTargetPTZF.zoom != targetPTZF.zoom) {
			gotoZoom(cameraSendChan, camAddr, targetPTZF.zoom)
		}
	} else {
		if( (oldTargetPTZF.zoom != targetPTZF.zoom) || (oldTargetPTZF.focus != targetPTZF.focus) ) {
			gotoZoomFocus(cameraSendChan, camAddr, targetPTZF.zoom, targetPTZF.focus)
		}
	}
	if( (oldTargetPTZF.pan != targetPTZF.pan) || (oldTargetPTZF.tilt != targetPTZF.tilt) ) {
		gotoPanTilt(cameraSendChan, camAddr, panspeed, tiltspeed, targetPTZF.pan, targetPTZF.tilt) // panspeed, tiltspeed, pan, tilt
	}
	oldTargetPTZF = commandPTZF
}

func mainControlLoop() {
	for {
		loop2 = loop2 + 1
		// take care with these shared variables!
		// they are single-byte to avoid race issues
		// only write them in the joystick routine
		// read them here and watch for changes
		//log.Println("loop ", loop1, " ", loop2 , " ", loop3, " ", loop4)
		time.Sleep(time.Millisecond*125)
		percentDone, triggerNow := checkTimerFraction()
		if(triggerNow) {
			log.Println("TIMER JUST EXPIRED!!!")
		}
		if percentDone < 1.0 {
			commandPTZF = interpolatePTZ(percentDone, startPTZF, endPTZF)
			gotoPTZF(commandPTZF)
			fmt.Println(commandPTZF)
		}

// this test code creates a race condition-induced crash, so it's helpful only to see what the values are in real time
/*			hatcoordinates := make([]float32, 4)
		for hatnum:=0; hatnum < 4; hatnum++ {
			if device.HatExists(uint8(hatnum)) {
				log.Println("Hat number: ", strconv.Itoa(hatnum))
				device.HatCoords(uint8(hatnum), hatcoordinates) // 3 is right hat vertical axis
				log.Println("Hat Coordinates: ", hatcoordinates)
			}
		}
		device.HatCoords(1, hatcoordinates)
		log.Println("Hat 1 Coordinates: ", hatcoordinates)
		device.HatCoords(3, hatcoordinates)
		log.Println("Hat 3 Coordinates: ", hatcoordinates) */
		if(oldpan != pan) {
			oldpan = pan
			log.Println("Pan is now:", oldpan)
			sendPanTiltSpeed(cameraSendChan, camAddr, speedLimit(deadBand(pan), slowPT), speedLimit(deadBand(tilt), slowPT))
		}
		if(oldtilt != tilt) {
			oldtilt = tilt
			log.Println("Tilt is now:", oldtilt)
			sendPanTiltSpeed(cameraSendChan, camAddr, speedLimit(deadBand(pan), slowPT), speedLimit(deadBand(tilt), slowPT))
		}
		if(oldSlowPT != slowPT) {
			oldSlowPT = slowPT
			log.Println("Tilt speed change")
			sendPanTiltSpeed(cameraSendChan, camAddr, speedLimit(deadBand(pan), slowPT), speedLimit(deadBand(tilt), slowPT))
		}
		if((oldzoom != zoom) || (oldSlowZ != slowZ)) {
			oldzoom = zoom
			oldSlowZ = slowZ
			if(slowZ) {
				log.Println("Zooming SLOWLY")
				if(zoom>0) {
					sendZoom(cameraSendChan, camAddr, 1)
				} else if(zoom<0) {
					sendZoom(cameraSendChan, camAddr, -1)
				} else {
					sendZoom(cameraSendChan, camAddr, 0)
				}
			} else {
				log.Println("Zoom is now:", oldzoom)
				sendZoom(cameraSendChan, camAddr, zoom)
			}
		}
		if(oldfocus != focus) {
			oldfocus = focus
			log.Println("Focus is now:", oldfocus)
			sendFocus(cameraSendChan, camAddr, focus)
		}
	}
	log.Println("exiting main control and execution for loop")
}
/*
var TestState TestStateT

func nullState() error {
        println(TestState.String())
        return nil
}
*/

func main() {
	log.Println("STARTING UP")
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGSTOP, syscall.SIGQUIT)

	go cameraComm(cameraSendChan, cameraReceiveChan, cameraErrChan)
	log.Println("cameraComm goroutine started!")
	go mainControlLoop()
	log.Println("mainControlLoop goroutine started!")

	// try connecting to specific controller.
	// the index is system assigned, typically it increments on each new controller added.
	// indexes remain fixed for a given controller, if/when other controller(s) are removed.
	log.Println("about to connect to the joystick")
	device := joysticks.Connect(1)
	log.Println("joysticks.Connect completed")

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
	log.Println("device.ParcelOutEvents goroutine started!")

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
				log.Println("Pos: ", hpos.X, "x, ", hpos.Y, "y")
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
				log.Println("button #5 pressed STOPPING PAN/TILT/ZOOM")
				tilt = 0
				oldtilt = 0
				pan = 0
				oldpan = 0
				sendPanTiltSpeed(cameraSendChan, camAddr, pan, tilt)
				zoom = 0
				oldzoom = 0
				sendZoom(cameraSendChan, camAddr, zoom)
				slowPT = false
				slowZ = false
			case <-b6press:
				log.Println("button #6 pressed SLOW PAN/TILT/ZOOM")
				slowPT = true
				slowZ = true
			case <-b7press:
				log.Println("button #7 pressed, requesting current Pan/Tilt values")
				getPanTilt(cameraSendChan, camAddr)
			case <-b8press:
				startTimer(60000)
				log.Println("button #8 pressed, starting slow pan")
			case <-b9press:
				gotoPTZFspeed(startPTZF, 24, 24)
				//gotoPanTilt(cameraSendChan, camAddr, 24, 24, -500, 0) // panspeed, tiltspeed, pan, tilt
				log.Println("button #9 pressed, going to pan start position")
			case <-b10press:
				log.Println("button #10 pressed")
			case <-b11press:
				log.Println("button #11 pressed")
			case <-b1release:
				log.Println("button #1 released")
			case <-b2release:
				log.Println("button #2 released")
			case <-b3release:
				log.Println("button #3 released")
			case <-b4release:
				log.Println("button #4 released")
			case <-b5release:
				log.Println("button #5 released STOPPING PAN/TILT/ZOOM")
				tilt = 0
				oldtilt = 0
				pan = 0
				oldpan = 0
				sendPanTiltSpeed(cameraSendChan, camAddr, pan, tilt)
				zoom = 0
				oldzoom = 0
				sendZoom(cameraSendChan, camAddr, zoom)
				slowPT = false
				slowZ = false
			case <-b6release:
				log.Println("button #6 released STOPPING PAN/TILT/ZOOM")
				tilt = 0
				oldtilt = 0
				pan = 0
				oldpan = 0
				sendPanTiltSpeed(cameraSendChan, camAddr, pan, tilt)
				zoom = 0
				oldzoom = 0
				sendZoom(cameraSendChan, camAddr, zoom)
				slowPT = false
				slowZ = false
			case <-b7release:
				log.Println("button #7 released")
			case <-b8release:
				log.Println("button #8 released")
			case <-b9release:
				log.Println("button #9 released")
			case <-b10release:
				log.Println("button #10 released")
			case <-b11release:
				log.Println("button #11 released")
			}
		}
		log.Println("exiting event capture goroutine")
	}()
	log.Println("event channels started!")

	mainrun := true
	for mainrun {
		select {
		case <-killSignal:
			mainrun = false
			log.Println("got kill signal!")
		case <-cameraErrChan:
			mainrun = false
			log.Println("camera communication error!")
		case <-controllerDisconnectChan:
			mainrun = false
			log.Println("USB joystick disconnect error!")
		case rxmsg := <-cameraReceiveChan:
			log.Println("Camera Response: ", viscaDecode(rxmsg))
	}
}

/*	gotoCloseShot()
	time.Sleep(100 * time.Millisecond) */
	log.Println("exiting!")
}
/*
func gotoWideShot () {
	gotoZoom(cameraSendChan, camAddr, 1000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -980, -180)
}

func gotoCloseShot () {
	gotoZoom(cameraSendChan, camAddr, 12500)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -982, -123)
}

func gotoLeftShot () {
	gotoZoom(cameraSendChan, camAddr, 8000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -1100, -123)
}

func gotoTempShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, camAddr, 14500)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -97, -132)
}

func gotoRightShot () {
	gotoZoom(cameraSendChan, camAddr, 8000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -860, -123)
}

func gotoPianoShot () {
	gotoZoom(cameraSendChan, camAddr, 12000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -1302, -100)
}

func gotoDirectorShot () {
	gotoZoom(cameraSendChan, camAddr, 12000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -950, -90)
}

func gotoOrganShot () {
	gotoZoom(cameraSendChan, camAddr, 12000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -915, -100)
}

func gotoCloseLeftShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, camAddr, 13000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -26, -95)
}

func gotoCloseRightShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, camAddr, 13000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, 14, -95)
}

func gotoMediumShot () {
	gotoZoom(cameraSendChan, camAddr, 5000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -980, -127)
}

func gotoMediumCloseShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, camAddr, 10500)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -6, -95)
}

func gotoChoirShot () {
	gotoZoom(cameraSendChan, camAddr, 7000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -1060, -100)
}

func gotoWideScreenShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, camAddr, 2000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -3, 65536-80)
}

func gotoScreenShot () { // TODO: Adjust
	gotoZoom(cameraSendChan, camAddr, 11000)
	gotoPanTilt(cameraSendChan, camAddr, 10, 10, -3, 20)
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
	go cameraRead(udpconn, cameraReceiveChan, cameraErrChan)
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

func cameraRead(conn net.Conn, cameraReceiveChan chan<- []byte, cameraErrChan chan<- bool) {
// Visca messages end in 0xFF, so use that as the termination character
// for reading responses back from the serial port (the 0xFF will be stripped)
	readbytes := make([]byte, 2048)
	run := true
	for (run) {
		loop3 = loop3 + 1
		log.Println("Starting UDP Read")
		bytecount, readerr := conn.Read(readbytes)
		cameraresponse := readbytes[:bytecount]
		cameraReceiveChan <- cameraresponse
		if (nil != readerr) {
			run = false
			log.Print(readerr)
		} else {
			log.Println("UDP Read completed successfully")
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

func getPanTilt(cameraSendChan chan<- []byte, cam byte) {
//	0x8x 0x09 0x06 0x12 0xFF (query)
	sendVisca(cameraSendChan, []byte{(0x80+cam), 0x09, 0x06, 0x12, 0xFF})
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

func gotoPanTilt(cameraSendChan chan<- []byte, cam byte, panspeed int16, tiltspeed int16, pan int16, tilt int16) {
	// Direct pan and tilt command at specific speed
	var m, n byte
	if(panspeed>24) {panspeed = 24}
	if(panspeed<(-24)) {panspeed = -24}
	if(panspeed>=0) {m=byte(panspeed)} else {m=byte(0-panspeed)}
	if(tiltspeed>20) {tiltspeed = 20}
	if(tiltspeed<(-20)) {tiltspeed = 20}
	if(tiltspeed>=0) {n=byte(tiltspeed)} else {n=byte(0-tiltspeed)}
	p := 0x0F & byte(uint16(pan) >> 12)
	q := 0x0F & byte(uint16(pan) >> 8)
	r := 0x0F & byte(uint16(pan) >> 4)
	s := 0x0F & byte(uint16(pan))
	t := 0x0F & byte(uint16(tilt) >> 12)
	u := 0x0F & byte(uint16(tilt) >> 8)
	v := 0x0F & byte(uint16(tilt) >> 4)
	w := 0x0F & byte(uint16(tilt))
	sendVisca(cameraSendChan, []byte{(0x80+cam), 0x01, 0x06, 0x02, m, n, p, q, r, s, t, u, v, w, 0xFF})
}

func sendPanTiltSpeed(cameraSendChan chan<- []byte, cam byte, pan int8, tilt int8) {
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

func deadBand(speed int8) (deadBandedSpeed int8) {
	if ((speed <= 1) && (speed >= -1)) {
		return 0
	}
	if (speed < -1) {
		return speed + 1
	} else {
		return speed - 1
	}
}

func viscaDecode(rxmsg []byte) (decResp string) {
	if 0xFF != rxmsg[len(rxmsg)-1] {
		return "INVALID! VISCA messages must end with 0xFF, not " + fmt.Sprintf("%02x",rxmsg[len(rxmsg)-1])
	}
	if 0x40 == (0xF0 & rxmsg[1]) {
		return "Camera ACK"
	}
	if 0x50 == (0xF0 & rxmsg[1]) {
		if 3 == len(rxmsg) {
			return "Camera command COMPLETED"
		} else {
			if 11 == len(rxmsg) { // probably a response to the request for pan/tilt position
				oredbytes := byte(0)
				for bytepos:=2; bytepos<10; bytepos++ {
					oredbytes |= rxmsg[bytepos]
				}
				if (0 == (0xF0 & oredbytes)) {
					pan := glueNibblesToInt(rxmsg[2:6])
					tilt := glueNibblesToInt(rxmsg[6:10])
					return "Pan position: " + strconv.Itoa(pan) + " and Tilt position: " + strconv.Itoa(tilt)
				}
			}
			return "Camera inquiry response: " + hex.Dump(rxmsg)
		}
	}
	if 0x60 == (0xF0 & rxmsg[1]) {
		if 1 == rxmsg[2] {
			return "ERROR: Bad Message Length"
		}
		if 2 == rxmsg[2] {
			return "ERROR: Bad Message Syntax"
		}
		if 3 == rxmsg[2] {
			return "ERROR: Command Buffer Full"
		}
		if 4 == rxmsg[2] {
			return "ERROR: Command Canceled"
		}
		if 5 == rxmsg[2] {
			return "ERROR: No Socket"
		}
		if 0x41 == rxmsg[2] {
			return "ERROR: Command Not Executable"
		}
	}
	return "unknown response type, raw: " + hex.Dump(rxmsg)
}

func glueNibblesToInt(nibbles []byte) (gluedInt int) {
	var gluedInt32 int32
	slicedNibbles := []byte{0,0,0,0} // decode VISCA bytes broken into nibbles
	if((0x08 & nibbles[0]) > 0) { // sign extension
		slicedNibbles[0] = 0xFF
		slicedNibbles[1] = 0xFF
	}
	slicedNibbles[2] = ((nibbles[0] & 0x0F) << 4) | (nibbles[1] & 0x0F)
	slicedNibbles[3] = ((nibbles[2] & 0x0F) << 4) | (nibbles[3] & 0x0F)
	nibbleBuf := bytes.NewBuffer(slicedNibbles)
	binary.Read(nibbleBuf, binary.BigEndian, &gluedInt32)
	gluedInt = int(gluedInt32)
	return gluedInt
}

// Read Pan Tilt Position
// 0x8x 0x09 0x06 0x12 0xFF (query)
// 0xy0 0x50 0x0p 0x0q 0x0r 0x0s 0x0t 0x0u 0x0v 0x0w 0xFF (response)
// 0xpqrs - pan position
// 0xtuvw - tilt position

// VISCA command references:
// https://www.epiphan.com/userguides/LUMiO12x/Content/UserGuides/PTZ/3-operation/VISCAcommands.htm