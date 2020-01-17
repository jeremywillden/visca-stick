package main

import "go.bug.st/serial.v1"
import "log"

func main () {

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

n, err := port.Write([]byte{0x88, 0x01, 0x04, 0x07, 0x02, 0xFF})
_ = n
if err != nil {
	log.Fatal(err)
}
//buff := make([]byte, 100)
for {

}

}
