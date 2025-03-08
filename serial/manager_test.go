package serial

import (
	"fmt"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/ventcon/ventcon-hwio/encoding"
)

type testSerial struct {
	isOpen      bool
	wasClosed   bool
	frames      []encoding.Frame
	failOnOpen  bool
	failOnClose bool
}

func (s *testSerial) Open(portName string) error {
	if s.failOnOpen {
		return fmt.Errorf("Some opening failure")
	}
	s.isOpen = true
	return nil
}
func (s *testSerial) Close() error {
	if s.failOnClose {
		return fmt.Errorf("Some closing failure")
	}
	s.wasClosed = true
	return nil
}
func (s *testSerial) SendRequest(data encoding.Frame) (encoding.Frame, error) {
	if data.FrameType() != encoding.ReadRequest {
		return nil, fmt.Errorf("Some sending failure")
	}
	if !s.isOpen {
		return nil, fmt.Errorf("Serial is not open")
	}
	s.frames = append(s.frames, data)
	return data, nil
}
func (s *testSerial) markAsValidSerial() {}

func TestNewSerialManager(t *testing.T) {
	managerInterface, requstChan, err := NewSerialManager("testPort")

	must.NoError(t, err)

	managerStruct, ok := managerInterface.(*serialManager)
	if !ok {
		t.Error("Returned serial manager interface is not a serial manager struct")
	}

	test.Eq(t, "testPort", managerStruct.port)
	test.NotNil(t, managerStruct.serial)
	test.NotNil(t, managerStruct.requests)
	test.NotNil(t, managerStruct.stop)
	test.NotNil(t, requstChan)

	req, err := encoding.NewReadRequest(100, 100)
	must.NoError(t, err)

	testReq := Request{
		ResponseChannel: nil,
		Data:            req,
	}
	go func() {
		requstChan <- testReq
	}()

	test.Eq(t, testReq, <-managerStruct.requests)

	close(requstChan)
	close(managerStruct.stop)
}

func setupTestSerialManager(t *testing.T) (*serialManager, chan<- Request, *testSerial) {
	managerInterface, requestChannel, err := NewSerialManager("testPort")
	must.NoError(t, err)

	managerStruct, ok := managerInterface.(*serialManager)
	if !ok {
		t.Error("Returned serial manager interface is not a serial manager struct")
	}

	serial := &testSerial{}
	managerStruct.serial = serial

	return managerStruct, requestChannel, serial
}

func isChannelClose[K any](ch <-chan K) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

func TestRunClosingRequestChannel(t *testing.T) {
	serialManager, requestChannel, serial := setupTestSerialManager(t)

	err := serialManager.Start()
	must.NoError(t, err)

	close(requestChannel)

	test.False(t, serial.wasClosed)
	test.False(t, isChannelClose(serialManager.stop))
	test.True(t, isChannelClose(serialManager.requests))

	err = serialManager.Stop()
	test.NoError(t, err)

	test.True(t, serial.wasClosed)
	test.True(t, isChannelClose(serialManager.stop))
	test.True(t, isChannelClose(serialManager.requests))
}

func TestRunSerialOpenFailure(t *testing.T) {
	serialManager, requestChannel, serial := setupTestSerialManager(t)

	serial.failOnOpen = true

	err := serialManager.Start()
	test.ErrorContains(t, err, "Some opening failure")

	test.True(t, isChannelClose(serialManager.stop))
	test.False(t, isChannelClose(serialManager.requests))

	close(requestChannel)
}

func TestRunSerialCloseFailure(t *testing.T) {
	serialManager, requestChannel, serial := setupTestSerialManager(t)

	serial.failOnClose = true

	err := serialManager.Start()
	must.NoError(t, err)

	test.False(t, serial.wasClosed)
	test.False(t, isChannelClose(serialManager.stop))
	test.False(t, isChannelClose(serialManager.requests))

	err = serialManager.Stop()
	test.ErrorContains(t, err, "Some closing failure")

	test.True(t, isChannelClose(serialManager.stop))
	test.False(t, isChannelClose(serialManager.requests))

	close(requestChannel)
}

type testRequest struct {
	responseChannel    chan Response
	num                int
	request            Request
	hasResponseChannel bool
	expectErr          bool
	expectFrameWritten bool
}

func mkTestRequest(t *testing.T, num int, isWrite bool, hasResponseChannel bool, hasData bool) testRequest {
	responseChannel := make(chan Response)

	var data encoding.Frame = nil
	var responseChannelForRequest chan Response = nil

	if hasData {
		var err error
		if isWrite {
			data, err = encoding.NewWriteRequest(num, num, num)
		} else {
			data, err = encoding.NewReadRequest(num, num)
		}
		must.NoError(t, err)
	}

	if hasResponseChannel {
		responseChannelForRequest = responseChannel
	}

	req := Request{
		ResponseChannel: responseChannelForRequest,
		Data:            data,
	}

	return testRequest{
		responseChannel:    responseChannel,
		num:                num,
		request:            req,
		hasResponseChannel: hasResponseChannel,
		expectErr:          isWrite && hasData && hasResponseChannel,
		expectFrameWritten: !isWrite && hasData && hasResponseChannel,
	}
}

func TestRun(t *testing.T) {
	for _, subset := range [][]int{{0, 1, 2, 3, 4, 5, 6, 7}, {0, 2, 4, 6}, {1, 3, 5, 7}, {0, 1, 2, 3}, {4, 5, 6, 7}, {0}, {1}, {2}, {3}, {4}, {5}, {6}, {7}} {
		t.Run(fmt.Sprintf(`TestCases(%v)`, subset), func(t *testing.T) {
			testRequests := []testRequest{
				mkTestRequest(t, 1, false, true, true),
				mkTestRequest(t, 2, false, true, false),
				mkTestRequest(t, 3, false, false, true),
				mkTestRequest(t, 4, false, false, false),
				mkTestRequest(t, 5, true, true, true),
				mkTestRequest(t, 6, true, true, false),
				mkTestRequest(t, 7, true, false, true),
				mkTestRequest(t, 8, true, false, false),
			}
			serialManager, requestChannel, serial := setupTestSerialManager(t)

			err := serialManager.Start()
			must.NoError(t, err)

			responses := make([]Response, len(testRequests))

			for _, i := range subset {
				request := testRequests[i]
				requestChannel <- request.request
				if request.hasResponseChannel {
					responses[i] = <-request.responseChannel
				}
			}

			writtenFrameCounter := 0

			for _, i := range subset {
				t.Run(fmt.Sprintf(`TestCase(%d): %d`, i, testRequests[i].num), func(t *testing.T) {
					request := testRequests[i]
					if request.expectFrameWritten {
						must.True(t, writtenFrameCounter < len(serial.frames))
						test.Eq(t, request.request.Data, serial.frames[writtenFrameCounter])
						writtenFrameCounter++
					}
					if request.hasResponseChannel {
						resp := responses[i]
						if request.expectErr {
							test.ErrorContains(t, resp.err, "Some sending failure")
						} else {
							must.NoError(t, resp.err)
							test.Eq(t, request.request.Data, resp.response)
						}
					}
				})
			}

			err = serialManager.Stop()
			must.NoError(t, err)

			test.True(t, serial.wasClosed)
			test.True(t, isChannelClose(serialManager.stop))
			test.False(t, isChannelClose(serialManager.requests))
		})
	}
}
