package serial

import (
	"bufio"
	"context"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/ventcon/ventcon-hwio/encoding"
	"go.bug.st/serial"

	log "github.com/sirupsen/logrus"
)

type Serial interface {
	Open(portName string) error
	Close() error
	SendRequest(ctx context.Context, data encoding.Frame) (encoding.Frame, error)
	markAsValidSerial()
}

type serialCommunicator struct {
	encoder *encoding.SerialEncoder
	port    serial.Port
	reader  *bufio.Reader
}

func NewSerial() (Serial, error) {
	encoder, err := encoding.NewSerialEncoder()
	if err != nil {
		return nil, err
	}
	return &serialCommunicator{encoder: encoder}, nil
}

func (serialCommunicator *serialCommunicator) Open(portName string) error {
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.EvenParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	log.WithFields(log.Fields{
		"portName":   portName,
		"serialMode": mode,
	}).Debug("Opening serial port")

	port, err := serial.Open(portName, mode)
	if err != nil {
		return merry.Prependf(err, "Failed to open serial connection for portName %s.", portName)
	}

	err = port.SetReadTimeout(20 * time.Millisecond)
	if err != nil {
		wrappedErr := merry.Prependf(err, "Failed to set the read timeout for portName %s.", portName)
		closeErr := port.Close()
		if closeErr != nil {
			return merry.WithValue("errorOnClose", closeErr).Wrap(wrappedErr, 1)
		}
		return wrappedErr
	}

	serialCommunicator.port = port
	serialCommunicator.reader = bufio.NewReader(port)
	return nil
}

func (serialCommunicator *serialCommunicator) Close() error {
	log.Debug("Closing serial port")
	if serialCommunicator.port != nil {
		return serialCommunicator.port.Close()
	}
	return nil
}

func (serialCommunicator *serialCommunicator) WriteFrame(ctx context.Context, data encoding.Frame) error {
	dataStr, err := serialCommunicator.encoder.Encode(data)
	if err != nil {
		return err
	}
	dataBytes := []byte(dataStr)

	deadline, ok := ctx.Deadline()
	if ok && deadline.After(time.Now()) {
		return merry.Prependf(ctx.Err(), "Aborted preparation for sending the following message (did not begin sending): %s", dataStr)
	}

	log.WithField("frame", data).Trace("Writing frame")

	_, err = serialCommunicator.port.Write(dataBytes)
	return merry.Prependf(err, "Failed to send serial message: %s", dataStr)
}

func (serialCommunicator *serialCommunicator) ReadFrame(ctx context.Context) (encoding.Frame, error) {
	str, err := serialCommunicator.reader.ReadString(encoding.CHAR_CR)
	if err != nil {
		return nil, merry.Prependf(err, "Failed to read full frame from serial port", merry.WithValue("readString", str))
	}
	log.WithField("data", encoding.DataWithEscapeChars(str)).Trace("Read full frame")
	return serialCommunicator.encoder.Decode(str)
}

func (serialCommunicator *serialCommunicator) SendRequest(ctx context.Context, data encoding.Frame) (encoding.Frame, error) {
	if err := serialCommunicator.WriteFrame(ctx, data); err != nil {
		return nil, merry.Prepend(err, "Failed to write request frame")
	}
	resp, err := serialCommunicator.ReadFrame(ctx)
	return resp, merry.Prepend(err, "Failed to read response frame")
}

func (serialCommunicator *serialCommunicator) markAsValidSerial() { /*Intentionally empty*/ }
