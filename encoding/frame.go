package encoding

import "github.com/ansel1/merry/v2"

const (
	MINIMUM_ADDRESS  = 1
	MAXIMUM_ADDRESS  = 250
	MINIMUM_FUNCTION = 0
	MAXIMUM_FUNCTION = 999
	MINIMUM_VALUE    = 0
	MAXIMUM_VALUE    = 999
)

// FrameType is the type of a serial frame
type FrameType string

const (
	// ReadRequest the is the FrameType indicating the frame requests a read operation
	ReadRequest FrameType = "readRequest"
	// WriteRequest the is the FrameType indicating the frame requests a write operation
	WriteRequest FrameType = "writeRequest"
	// ReadResponse the is the FrameType indicating the frame is a response to a read operation
	ReadResponse FrameType = "readResponse"
	// WriteResponse the is the FrameType indicating the frame is a response to a write operation
	WriteResponse FrameType = "writeResponse"
)

// frame represents a single request or response send over the serial interface
type frame struct {
	// frameType is the type of the frame
	frameType FrameType
	// address is the address of the ventilator this frame is for/from
	address uint16
	// function is the function of the ventilator to be read/written or that was read from / written to
	function uint16
	// value is the value to be written or the value that was read/written
	value uint16
}

// FrameType gets the type of the frame
func (f frame) FrameType() FrameType {
	return f.frameType
}

// Adress gets the address of the ventilator this frame is for/from
func (f frame) Address() int {
	return int(f.address)
}

// Function gets the function of the ventilator to be read/written or that was read from / written to
func (f frame) Function() int {
	return int(f.function)
}

// Value gets the value to be written or the value that was read/written
func (f frame) Value() int {
	return int(f.value)
}

func (f frame) markAsValidFrame() { /*Intentionally empty*/ }

// Frame represents a single request or response send over the serial interface
type Frame interface {
	FrameType() FrameType
	Address() int
	Function() int
	Value() int
	markAsValidFrame()
}

// NewReadRequest creates a new read request
func NewReadRequest(address int, function int) (Frame, error) {
	if address < MINIMUM_ADDRESS || address > MAXIMUM_ADDRESS {
		return nil, merry.Errorf("The address must be between %d and %d (inclusive). It was %d", MINIMUM_ADDRESS, MAXIMUM_ADDRESS, address)
	}
	if function < MINIMUM_FUNCTION || function > MAXIMUM_FUNCTION {
		return nil, merry.Errorf("The function must be between %d and %d (inclusive). It was %d", MINIMUM_FUNCTION, MAXIMUM_FUNCTION, function)
	}
	return &frame{frameType: ReadRequest, address: uint16(address), function: uint16(function)}, nil
}

// NewWriteRequest creates a new Write request
func NewWriteRequest(address int, function int, value int) (Frame, error) {
	if address < MINIMUM_ADDRESS || address > MAXIMUM_ADDRESS {
		return nil, merry.Errorf("The address must be between %d and %d (inclusive). It was %d", MINIMUM_ADDRESS, MAXIMUM_ADDRESS, address)
	}
	if function < MINIMUM_FUNCTION || function > MAXIMUM_FUNCTION {
		return nil, merry.Errorf("The function must be between %d and %d (inclusive). It was %d", MINIMUM_FUNCTION, MAXIMUM_FUNCTION, function)
	}
	if value < MINIMUM_VALUE || value > MAXIMUM_VALUE {
		return nil, merry.Errorf("The value must be between %d and %d (inclusive). It was %d", MINIMUM_VALUE, MAXIMUM_VALUE, value)
	}
	return &frame{frameType: WriteRequest, address: uint16(address), function: uint16(function), value: uint16(value)}, nil
}

// newResponse creates a new response
func newReponse(frameType FrameType, address uint16, function uint16, value uint16) (*frame, error) {
	if !(frameType == ReadResponse || frameType == WriteResponse) {
		return nil, merry.Errorf("Invalid frame type for a response: %s", frameType)
	}
	if address < MINIMUM_ADDRESS || address > MAXIMUM_ADDRESS {
		return nil, merry.Errorf("The address must be between %d and %d (inclusive). It was %d", MINIMUM_ADDRESS, MAXIMUM_ADDRESS, address)
	}
	if function < MINIMUM_FUNCTION || function > MAXIMUM_FUNCTION {
		return nil, merry.Errorf("The function must be between %d and %d (inclusive). It was %d", MINIMUM_FUNCTION, MAXIMUM_FUNCTION, function)
	}
	if value < MINIMUM_VALUE || value > MAXIMUM_VALUE {
		return nil, merry.Errorf("The value must be between %d and %d (inclusive). It was %d", MINIMUM_VALUE, MAXIMUM_VALUE, value)
	}
	return &frame{frameType: frameType, address: address, function: function, value: value}, nil
}
