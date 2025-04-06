package serial

import (
	"github.com/ventcon/ventcon-hwio/encoding"

	log "github.com/sirupsen/logrus"
)

type Response struct {
	Response encoding.Frame
	Err      error
}

type Request struct {
	ResponseChannel chan<- Response
	Data            encoding.Frame
}

type SerialManager interface {
	Start() error
	Stop() error
	markAsValidSerialManager()
}

type serialManager struct {
	port     string
	serial   Serial
	requests <-chan Request
	stop     chan (chan<- error)
}

func NewSerialManager(port string) (SerialManager, chan<- Request, error) {
	serial, err := NewSerial()
	if err != nil {
		return nil, nil, err
	}
	requests := make(chan Request)
	return &serialManager{
		port:     port,
		serial:   serial,
		requests: requests,
		stop:     make(chan (chan<- error)),
	}, requests, nil
}

func (serialManager *serialManager) Start() error {
	log.Debug("Starting serial manager for ", serialManager.port)
	err := serialManager.serial.Open(serialManager.port)
	if err != nil {
		close(serialManager.stop)
		return err
	}
	go func() {
		for {
			select {
			case stopResult := <-serialManager.stop:
				stopResult <- serialManager.serial.Close()
				return
			case request, ok := <-serialManager.requests:
				if !ok {
					// The channel has been closed. Wait for a stop signal.
					stopResult := <-serialManager.stop
					stopResult <- serialManager.serial.Close()
					return
				}
				if request.Data != nil && request.ResponseChannel != nil {
					response, err := serialManager.serial.SendRequest(request.Data)
					request.ResponseChannel <- Response{response, err}
				}
				if request.ResponseChannel != nil {
					close(request.ResponseChannel)
				}
			}
		}
	}()
	return nil
}

func (serialManager *serialManager) Stop() error {
	log.Debug("Stopping serial manager for ", serialManager.port)
	stopResult := make(chan error)
	serialManager.stop <- stopResult
	close(serialManager.stop)
	return <-stopResult
}

func (serialManager *serialManager) markAsValidSerialManager() {}
