package encoding

import (
	"fmt"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

const (
	DEFAULT_ADDRESS  = 100
	DEFAULT_FUNCTION = 200
	DEFAULT_VALUE    = 300
)

// testFrameValues tests whether all the values of the given frame match the given expected values
func testFrameValues(t *testing.T, frame Frame, address int, frameType FrameType, function int, value int) {
	must.NotNil(t, frame)
	test.EqOp(t, address, frame.Address())
	test.EqOp(t, frameType, frame.FrameType())
	test.EqOp(t, function, frame.Function())
	test.EqOp(t, value, frame.Value())
}

type frameTestCase struct {
	address  int
	function int
	value    int
}

type frameTestCaseDecider func(frameTestCase) bool

func testCasesForCombiantionsOfExcept(addresses []int, functions []int, values []int, exclude frameTestCaseDecider) []frameTestCase {
	result := make([]frameTestCase, 0, len(addresses)*len(functions)*len(values))
	for _, address := range addresses {
		for _, function := range functions {
			for _, value := range values {
				newTestCase := frameTestCase{address: address, function: function, value: value}
				if !exclude(newTestCase) {
					result = append(result, frameTestCase{address: address, function: function, value: value})
				}
			}
		}
	}
	return result
}

func TestNewReadRequestGood(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS, MAXIMUM_ADDRESS}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION, MAXIMUM_FUNCTION}, []int{0}, func(tc frameTestCase) bool {
		return false
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`NewReadRequest(%d, %d)`, tc.address, tc.function), func(t *testing.T) {
			frame, err := NewReadRequest(tc.address, tc.function)
			must.NoError(t, err)
			testFrameValues(t, frame, tc.address, ReadRequest, tc.function, 0)
		})
	}
}

func TestNewReadRequestBad(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS - 1, MAXIMUM_ADDRESS + 1}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION - 1, MAXIMUM_FUNCTION + 1}, []int{0}, func(tc frameTestCase) bool {
		return tc.address == DEFAULT_ADDRESS && tc.function == DEFAULT_FUNCTION
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`NewReadRequest(%d, %d)`, tc.address, tc.function), func(t *testing.T) {
			_, err := NewReadRequest(tc.address, tc.function)
			test.Error(t, err)
		})
	}
}

func TestNewWriteRequestGood(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS, MAXIMUM_ADDRESS}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION, MAXIMUM_FUNCTION}, []int{DEFAULT_VALUE, MINIMUM_VALUE, MAXIMUM_VALUE}, func(tc frameTestCase) bool {
		return false
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`NewWriteRequest(%d, %d, %d)`, tc.address, tc.function, tc.value), func(t *testing.T) {
			frame, err := NewWriteRequest(tc.address, tc.function, tc.value)
			must.NoError(t, err)
			testFrameValues(t, frame, tc.address, WriteRequest, tc.function, tc.value)
		})
	}
}

func TestNewWriteRequestBad(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS - 1, MAXIMUM_ADDRESS + 1}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION - 1, MAXIMUM_FUNCTION + 1}, []int{DEFAULT_VALUE, MINIMUM_VALUE - 1, MAXIMUM_VALUE + 1}, func(tc frameTestCase) bool {
		return tc.address == DEFAULT_ADDRESS && tc.function == DEFAULT_FUNCTION && tc.value == DEFAULT_VALUE
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`NewWriteRequest(%d, %d, %d)`, tc.address, tc.function, tc.value), func(t *testing.T) {
			_, err := NewWriteRequest(tc.address, tc.function, tc.value)
			test.Error(t, err)
		})
	}
}

func TestNewReponseWriteGood(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS, MAXIMUM_ADDRESS}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION, MAXIMUM_FUNCTION}, []int{DEFAULT_VALUE, MINIMUM_VALUE, MAXIMUM_VALUE}, func(tc frameTestCase) bool {
		return false
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`newResponse(WriteResponse, %d, %d, %d)`, tc.address, tc.function, tc.value), func(t *testing.T) {
			frame, err := newReponse(WriteResponse, uint16(tc.address), uint16(tc.function), uint16(tc.value))
			must.NoError(t, err)
			testFrameValues(t, frame, tc.address, WriteResponse, tc.function, tc.value)
		})
	}
}

func TestNewReponseReadGood(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS, MAXIMUM_ADDRESS}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION, MAXIMUM_FUNCTION}, []int{DEFAULT_VALUE, MINIMUM_VALUE, MAXIMUM_VALUE}, func(tc frameTestCase) bool {
		return false
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`newResponse(ReadResponse, %d, %d, %d)`, tc.address, tc.function, tc.value), func(t *testing.T) {
			frame, err := newReponse(ReadResponse, uint16(tc.address), uint16(tc.function), uint16(tc.value))
			must.NoError(t, err)
			testFrameValues(t, frame, tc.address, ReadResponse, tc.function, tc.value)
		})
	}
}

func TestNewReponseWriteBad(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS - 1, MAXIMUM_ADDRESS + 1}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION - 1, MAXIMUM_FUNCTION + 1}, []int{DEFAULT_VALUE, MINIMUM_VALUE - 1, MAXIMUM_VALUE + 1}, func(tc frameTestCase) bool {
		return tc.address == DEFAULT_ADDRESS && tc.function == DEFAULT_FUNCTION && tc.value == DEFAULT_VALUE
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`newResponse(WriteResponse, %d, %d, %d)`, tc.address, tc.function, tc.value), func(t *testing.T) {
			_, err := newReponse(WriteResponse, uint16(tc.address), uint16(tc.function), uint16(tc.value))
			test.Error(t, err)
		})
	}
}

func TestNewReponseReadBad(t *testing.T) {
	testCases := testCasesForCombiantionsOfExcept([]int{DEFAULT_ADDRESS, MINIMUM_ADDRESS - 1, MAXIMUM_ADDRESS + 1}, []int{DEFAULT_FUNCTION, MINIMUM_FUNCTION - 1, MAXIMUM_FUNCTION + 1}, []int{DEFAULT_VALUE, MINIMUM_VALUE - 1, MAXIMUM_VALUE + 1}, func(tc frameTestCase) bool {
		return tc.address == DEFAULT_ADDRESS && tc.function == DEFAULT_FUNCTION && tc.value == DEFAULT_VALUE
	})

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`newResponse(ReadResponse, %d, %d, %d)`, tc.address, tc.function, tc.value), func(t *testing.T) {
			_, err := newReponse(ReadResponse, uint16(tc.address), uint16(tc.function), uint16(tc.value))
			test.Error(t, err)
		})
	}
}

func TestNewReponseBadFrameType(t *testing.T) {
	testCases := []struct {
		frameType FrameType
	}{
		{ReadRequest},
		{WriteRequest},
		{"foo"},
		{""},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`newResponse(%s, 100, 200, 300)`, tc.frameType), func(t *testing.T) {
			_, err := newReponse(tc.frameType, uint16(100), uint16(200), uint16(300))
			test.Error(t, err)
		})
	}
}
