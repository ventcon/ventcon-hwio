package serial

import (
	"context"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/ventcon/ventcon-hwio/encoding"
	"go.bug.st/serial"
)

type serialCommunicator struct {
	encoder encoding.SerialEncoder
	port    serial.Port
}

func NewSerialCommunicator() (*serialCommunicator, error) {
	encoder, err := encoding.NewSerialEncoder()
	if err != nil {
		return nil, err
	}
	return &serialCommunicator{encoder: *encoder}, nil
}

func (serialCommunicator serialCommunicator) Open(portName string) error {
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.EvenParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(portName, mode)
	if err != nil {
		return merry.Prependf(err, "Failed to open serial connection for portName %s.", portName)
	}

	port.SetReadTimeout(10 * time.Millisecond)
	if err != nil {
		return merry.Prependf(err, "Failed to open serial connection for portName %s.", portName)
	}
	serialCommunicator.port = port
	return nil
}

func (serialCommunicator serialCommunicator) write(ctx context.Context, data encoding.Frame) error {
	dataStr, err := serialCommunicator.encoder.Encode(data)
	if err != nil {
		return err
	}
	dataBytes := []byte(dataStr)

	deadline, ok := ctx.Deadline()
	if ok && deadline.After(time.Now()) {
		return merry.Prependf(ctx.Err(), "Aborted preparation for sending the following message (did not begin sending): %s", dataStr)
	}

	_, err = serialCommunicator.port.Write(dataBytes)
	return merry.Prependf(err, "Failed to send serial message: %s", dataStr)
}

func (serialCommunicator serialCommunicator) readOnce() (string, error) {
	buff := make([]byte, 16)
	_, err := serialCommunicator.port.Read(buff)
	return string(buff), err
}

func (serialCommunicator serialCommunicator) read(ctx context.Context) (encoding.Frame, error) {
	buff := make([]byte, 16)

	serialCommunicator.port.Read()
}
