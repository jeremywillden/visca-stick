// log a description of events when pressing button #1 or moving hat#1. 
// 10sec timeout.
package main

import . "github.com/splace/joysticks"
import "log"
import "time"
import "math"

func main() {
	var pan int8 = 0
	var tilt int8 = 0
	var zoom int8 = 0
	var focus int8 = 0
	// try connecting to specific controller.
	// the index is system assigned, typically it increments on each new controller added.
	// indexes remain fixed for a given controller, if/when other controller(s) are removed.
	device := Connect(1)

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

	// start feeding OS events onto the event channels.
	go device.ParcelOutEvents()

	// handle event channels
	go func(){
		for{
			select {
			case h1 := <-h1move:
				hpos:=h1.(CoordsEvent)
				if(pan != int8(math.Floor(float64(7*hpos.X)))) {
					pan = int8(math.Floor(float64(7*hpos.X)))
					log.Println("Pan is now:", pan)
				}
				if(tilt != int8(math.Floor(float64(-7*hpos.Y)))) {
					tilt = int8(math.Floor(float64(-7*hpos.Y)))
					log.Println("Tilt is now:", tilt)
				}
			case h2 := <-h2move:
				hpos:=h2.(CoordsEvent)
//				if(pan != int8(math.Floor(float64(7*hpos.X)))) {
//					pan = int8(math.Floor(float64(7*hpos.X)))
//					log.Println("Pan is now:", pan)
//				}
				if(focus != int8(math.Floor(float64(7*hpos.Y)))) {
					focus = int8(math.Floor(float64(7*hpos.Y)))
					log.Println("Focus is now:", focus)
				}
			case h3 := <-h3move:
				hpos:=h3.(CoordsEvent)
				if(zoom != int8(math.Floor(float64(-7*hpos.X)))) {
					zoom = int8(math.Floor(float64(-7*hpos.X)))
					log.Println("Zoom is now:", zoom)
				}
//				if(tilt != int8(math.Floor(float64(7*hpos.Y)))) {
//					tilt = int8(math.Floor(float64(7*hpos.Y)))
//					log.Println("Tilt is now:", tilt)
//				}
			case h4 := <-h4move:
				hpos:=h4.(CoordsEvent)
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
	for {}

	log.Println("Timeout in 10 secs.")
	time.Sleep(time.Second*10)
	log.Println("Shutting down due to timeout.")
}
