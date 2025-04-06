package serial

import (
	"fmt"
	"testing"
	"time"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/ventcon/ventcon-hwio/encoding"
	"go.bug.st/serial"
)

const PORT_NAME = "/my/port/name"

type testSerialPort struct {
	serial.Port
	failOnSetReadTimeout bool
	failOnClose          bool
	failOnWrite          bool
	failOnRead           bool
	failOnReadButStart   bool
	readTimeout          time.Duration
	hasBeenClosed        bool
	written              []byte
	readData             []byte
	readOffset           int
}

func (sp *testSerialPort) Read(p []byte) (n int, err error) {
	if sp.failOnRead {
		return 0, fmt.Errorf("Some Read failure")
	}
	if sp.failOnReadButStart {
		if sp.readOffset == 0 {
			sp.readOffset = 1
			return copy(p, []byte("e")), nil
		}
		return 0, fmt.Errorf("Some Read failure")
	}
	dataLeft := len(sp.readData) - sp.readOffset
	toRead := dataLeft
	if len(p) < toRead {
		toRead = len(p)
	}

	return copy(p, sp.readData[sp.readOffset:sp.readOffset+toRead]), nil
}
func (sp *testSerialPort) Write(p []byte) (n int, err error) {
	if sp.failOnWrite {
		return 0, fmt.Errorf("Some Write failure")
	}
	toWrite := len(p)

	newWritten := make([]byte, len(sp.written), cap(sp.written)+toWrite)
	copy(newWritten, sp.written)
	newWritten = append(newWritten, p...)
	sp.written = newWritten

	return toWrite, nil
}
func (sp *testSerialPort) SetReadTimeout(t time.Duration) error {
	sp.readTimeout = t
	if sp.failOnSetReadTimeout {
		return fmt.Errorf("Some SetReadTimeout failure")
	}
	return nil
}
func (sp *testSerialPort) Close() error {
	sp.hasBeenClosed = true
	if sp.failOnClose {
		return fmt.Errorf("Some Close failure")
	}
	return nil
}

type testSerialEncoder struct{}

func (se *testSerialEncoder) Encode(frame encoding.Frame) (string, error) {
	return "", fmt.Errorf("Some Encode failure")
}

func (se *testSerialEncoder) Decode(data string) (encoding.Frame, error) {
	return nil, fmt.Errorf("Some Decode failure")
}

func TestNewSerial(t *testing.T) {
	serialInterface, err := NewSerial()
	must.NoError(t, err)

	serialCommunicator, ok := serialInterface.(*serialCommunicator)
	if !ok {
		t.Error("Returned serial interface is not a serial communicator")
	}

	test.NotNil(t, serialCommunicator.encoder)
	test.NotNil(t, serialCommunicator.lowLevelSerialOpener)
}

func TestOpenGood(t *testing.T) {
	serialInterface, err := NewSerial()
	must.NoError(t, err)
	serialCommunicator, ok := serialInterface.(*serialCommunicator)
	if !ok {
		t.Error("Returned serial interface is not a serial communicator")
	}
	testSp := &testSerialPort{}
	serialCommunicator.lowLevelSerialOpener =
		func(portName string, mode *serial.Mode) (serial.Port, error) {
			test.Eq(t, PORT_NAME, portName)
			test.Eq(t, 9600, mode.BaudRate)
			test.Eq(t, serial.EvenParity, mode.Parity)
			test.Eq(t, 8, mode.DataBits)
			return testSp, nil
		}

	err = serialInterface.Open(PORT_NAME)
	test.NoError(t, err)
	test.Eq(t, 20*time.Millisecond, testSp.readTimeout)
	test.Eq[serial.Port](t, testSp, serialCommunicator.port)
}

func TestOpenErrorOnOpen(t *testing.T) {
	serialInterface, err := NewSerial()
	must.NoError(t, err)
	serialCommunicator, ok := serialInterface.(*serialCommunicator)
	if !ok {
		t.Error("Returned serial interface is not a serial communicator")
	}
	serialCommunicator.lowLevelSerialOpener =
		func(portName string, mode *serial.Mode) (serial.Port, error) {
			return nil, fmt.Errorf("Some Open failure")
		}

	err = serialInterface.Open(PORT_NAME)
	test.ErrorContains(t, err, "Some Open failure")
}

func TestOpenErrorOnSetReadTimeout(t *testing.T) {
	serialInterface, err := NewSerial()
	must.NoError(t, err)
	serialCommunicator, ok := serialInterface.(*serialCommunicator)
	if !ok {
		t.Error("Returned serial interface is not a serial communicator")
	}
	testSp := &testSerialPort{
		failOnSetReadTimeout: true,
	}
	serialCommunicator.lowLevelSerialOpener =
		func(portName string, mode *serial.Mode) (serial.Port, error) {
			return testSp, nil
		}

	err = serialInterface.Open(PORT_NAME)
	test.ErrorContains(t, err, "Some SetReadTimeout failure")
	test.True(t, testSp.hasBeenClosed)
}

func TestOpenErrorOnSetReadTimeoutAndErrorOnClose(t *testing.T) {
	serialInterface, err := NewSerial()
	must.NoError(t, err)
	serialCommunicator, ok := serialInterface.(*serialCommunicator)
	if !ok {
		t.Error("Returned serial interface is not a serial communicator")
	}
	testSp := &testSerialPort{
		failOnSetReadTimeout: true,
		failOnClose:          true,
	}
	serialCommunicator.lowLevelSerialOpener =
		func(portName string, mode *serial.Mode) (serial.Port, error) {
			return testSp, nil
		}

	err = serialInterface.Open(PORT_NAME)
	errstr := fmt.Sprintf("%s", err)
	fmt.Print(errstr)
	test.ErrorContains(t, err, "Some SetReadTimeout failure")
	test.ErrorContains(t, err, "Some Close failure")
}

func setupWorkingCommunicator(t *testing.T, testSp *testSerialPort, open bool) *serialCommunicator {
	serialInterface, err := NewSerial()
	must.NoError(t, err)
	serialCommunicator, ok := serialInterface.(*serialCommunicator)
	if !ok {
		t.Error("Returned serial interface is not a serial communicator")
	}
	serialCommunicator.lowLevelSerialOpener =
		func(portName string, mode *serial.Mode) (serial.Port, error) {
			return testSp, nil
		}

	if open {
		err = serialInterface.Open(PORT_NAME)
		must.NoError(t, err)
	}
	return serialCommunicator
}

func TestWriteFrameGood(t *testing.T) {
	testSp := &testSerialPort{}
	serial := setupWorkingCommunicator(t, testSp, true)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	err = serial.WriteFrame(req)

	test.NoError(t, err)
	test.Eq(t, []byte("\n100lW100\r"), testSp.written)
}

func TestWriteFrameSerialPortNotYetOpen(t *testing.T) {
	testSp := &testSerialPort{}
	serial := setupWorkingCommunicator(t, testSp, false)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	err = serial.WriteFrame(req)

	test.ErrorContains(t, err, "Serial port not yet opened")
	test.Len(t, 0, testSp.written)
}

func TestWriteFrameEncodingFails(t *testing.T) {
	testSp := &testSerialPort{}
	serial := setupWorkingCommunicator(t, testSp, true)
	serial.encoder = &testSerialEncoder{}

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	err = serial.WriteFrame(req)

	test.ErrorContains(t, err, "Some Encode failure")
	test.Len(t, 0, testSp.written)
}

func TestWriteFrameFailOnWrite(t *testing.T) {
	testSp := &testSerialPort{
		failOnWrite: true,
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	err = serial.WriteFrame(req)

	test.ErrorContains(t, err, "Some Write failure")
	test.Len(t, 0, testSp.written)
}

func TestReadFrameGood(t *testing.T) {
	testSp := &testSerialPort{
		readData: []byte("\n111lW#222333\r"),
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	frame, err := serial.ReadFrame()

	test.NoError(t, err)
	test.Eq(t, encoding.ReadResponse, frame.FrameType())
	test.Eq(t, 111, frame.Address())
	test.Eq(t, 222, frame.Function())
	test.Eq(t, 333, frame.Value())
}

func TestReadFramePortNotYetOpen(t *testing.T) {
	testSp := &testSerialPort{
		readData: []byte("\n111lW#222333\r"),
	}
	serial := setupWorkingCommunicator(t, testSp, false)

	_, err := serial.ReadFrame()

	test.ErrorContains(t, err, "Serial port not yet opened")
	test.Eq(t, 0, testSp.readOffset)
}

func TestReadFrameFailOnRead(t *testing.T) {
	testSp := &testSerialPort{
		failOnRead: true,
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	_, err := serial.ReadFrame()

	test.ErrorContains(t, err, "Some Read failure")
	test.Eq(t, 0, testSp.readOffset)
}

func TestReadFrameNoData(t *testing.T) {
	testSp := &testSerialPort{}
	serial := setupWorkingCommunicator(t, testSp, true)

	_, err := serial.ReadFrame()

	test.ErrorIs(t, err, NoDataOnSerialError)
	test.Eq(t, 0, testSp.readOffset)
}

func TestReadFrameFailOnReadButStart(t *testing.T) {
	testSp := &testSerialPort{
		failOnReadButStart: true,
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	_, err := serial.ReadFrame()

	test.ErrorContains(t, err, "Some Read failure")
	test.Eq(t, 1, testSp.readOffset)
}

func TestReadFrameDecodingFails(t *testing.T) {
	testSp := &testSerialPort{
		readData: []byte("\n111lW#222333\r"),
	}
	serial := setupWorkingCommunicator(t, testSp, true)
	serial.encoder = &testSerialEncoder{}

	_, err := serial.ReadFrame()

	test.ErrorContains(t, err, "Some Decode failure")
}

func TestSendRequestGood(t *testing.T) {
	testSp := &testSerialPort{
		readData: []byte("\n111lW#222333\r"),
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	resp, err := serial.SendRequest(req)

	test.NoError(t, err)
	test.Eq(t, encoding.ReadResponse, resp.FrameType())
	test.Eq(t, 111, resp.Address())
	test.Eq(t, 222, resp.Function())
	test.Eq(t, 333, resp.Value())
}

func TestSendRequestWriteFails(t *testing.T) {
	testSp := &testSerialPort{
		readData:    []byte("\n111lW#222333\r"),
		failOnWrite: true,
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	_, err = serial.SendRequest(req)

	test.ErrorContains(t, err, "Some Write failure")
}

func TestSendRequestReadFails(t *testing.T) {
	testSp := &testSerialPort{
		readData:   []byte("\n111lW#222333\r"),
		failOnRead: true,
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	_, err = serial.SendRequest(req)

	test.ErrorContains(t, err, "Some Read failure")
}

func TestCloseGood(t *testing.T) {
	testSp := &testSerialPort{}
	serial := setupWorkingCommunicator(t, testSp, true)

	err := serial.Close()

	test.NoError(t, err)
	test.Eq(t, true, testSp.hasBeenClosed)
}

func TestCloseWithoutOpen(t *testing.T) {
	testSp := &testSerialPort{}
	serial := setupWorkingCommunicator(t, testSp, false)

	err := serial.Close()

	test.NoError(t, err)
	test.Eq(t, false, testSp.hasBeenClosed)
}

func TestCloseError(t *testing.T) {
	testSp := &testSerialPort{
		failOnClose: true,
	}
	serial := setupWorkingCommunicator(t, testSp, true)

	err := serial.Close()

	test.ErrorContains(t, err, "Some Close failure")
}
