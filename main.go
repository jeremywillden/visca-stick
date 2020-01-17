// log a description of events when pressing button #1 or moving hat#1. 
// 10sec timeout.
package main

import . "github.com/splace/joysticks"
import "log"
import "time"

func main() {
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
	h1move := device.OnMove(1)

	// start feeding OS events onto the event channels. 
	go device.ParcelOutEvents()

	// handle event channels
	go func(){
		for{
			select {
			case <-b1press:
				log.Println("button #1 pressed")
			case h := <-h1move:
				hpos:=h.(CoordsEvent)
				log.Println("hat #1 moved too:", hpos.X,hpos.Y)
			}
		}
	}()

	log.Println("Timeout in 10 secs.")
	time.Sleep(time.Second*10)
	log.Println("Shutting down due to timeout.")
}
