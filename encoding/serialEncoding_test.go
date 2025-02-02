package encoding

import (
	"fmt"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestNewSerialEncoder(t *testing.T) {
	encoder, err := NewSerialEncoder()
	must.NoError(t, err)

	test.NotNil(t, encoder.readTemplate)
	test.NotNil(t, encoder.writeTemplate)
	test.NotNil(t, encoder.responseRegex)
}

func TestEncodeReadRequest(t *testing.T) {
	encoder, err := NewSerialEncoder()
	must.NoError(t, err)

	testCases := []struct {
		address  int
		function int
		result   string
	}{
		{10, 20, "\n010lW020\r"},
		{1, 0, "\n001lW000\r"},
		{250, 999, "\n250lW999\r"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`Encode(ReadRequest(%d, %d)) == %s`, tc.address, tc.function, tc.result), func(t *testing.T) {
			frame, err := NewReadRequest(tc.address, tc.function)
			must.NoError(t, err)
			result, err := encoder.Encode(frame)
			must.NoError(t, err)
			test.EqOp(t, tc.result, result)
		})
	}
}

func TestEncodeWriteRequest(t *testing.T) {
	encoder, err := NewSerialEncoder()
	must.NoError(t, err)

	testCases := []struct {
		address  int
		function int
		value    int
		result   string
	}{
		{10, 20, 30, "\n010sW020030\r"},
		{1, 0, 0, "\n001sW000000\r"},
		{250, 999, 999, "\n250sW999999\r"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`Encode(WriteRequest(%d, %d, %d)) == %s`, tc.address, tc.function, tc.value, tc.result), func(t *testing.T) {
			frame, err := NewWriteRequest(tc.address, tc.function, tc.value)
			must.NoError(t, err)
			result, err := encoder.Encode(frame)
			must.NoError(t, err)
			test.EqOp(t, tc.result, result)
		})
	}
}

func TestDecodeResponseGood(t *testing.T) {
	encoder, err := NewSerialEncoder()
	must.NoError(t, err)

	testCases := []struct {
		address   int
		frameType FrameType
		function  int
		value     int
		input     string
	}{
		{10, ReadResponse, 20, 30, "\n010lW#020030\r"},
		{1, ReadResponse, 0, 0, "\n001lW#000000\r"},
		{250, ReadResponse, 999, 999, "\n250lW#999999\r"},
		{10, WriteResponse, 20, 30, "\n010sW#020030\r"},
		{1, WriteResponse, 0, 0, "\n001sW#000000\r"},
		{250, WriteResponse, 999, 999, "\n250sW#999999\r"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`Decode(%s) == Response(%d, %s, %d, %d)`, tc.input, tc.address, tc.frameType, tc.function, tc.value), func(t *testing.T) {
			frame, err := encoder.Decode(tc.input)
			must.NoError(t, err)
			testFrameValues(t, frame, tc.address, tc.frameType, tc.function, tc.value)
		})
	}
}

func TestDecodeResponseBad(t *testing.T) {
	encoder, err := NewSerialEncoder()
	must.NoError(t, err)

	testCases := []struct {
		input string
	}{
		//Regex really wrong errors
		{"\n01lW#020030\r"},  //Address only 2 chars
		{"\n010lW#02030\r"},  //Function only 2 chars
		{"\n010lW#02003\r"},  //Value only 2 chars
		{"\n010lW#020030"},   //Missing CR
		{"010lW#020030\r"},   //Missing NL
		{"\n010lW020030\r"},  //Missing #
		{"\n010l#020030\r"},  //Missing W
		{"\n010W#020030\r"},  //Missing response type
		{"\n\r"},             //Missing all but CR NL
		{""},                 //Empty String
		{"\n010lW#020\r"},    //Missing value or function
		{"\nlW#020030\r"},    //Missing address
		{"\n010l#W020030\r"}, //Wrong order
		{"\n010W#l020030\r"}, //Wrong order
		//Not an int errors (still detected by regex)
		{"\n01alW#020030\r"},
		{"\nX10lW#020030\r"},
		{"\n010lW#02a030\r"},
		{"\n010lW#x20030\r"},
		{"\n010lW#02003a\r"},
		{"\n010lW#020x30\r"},
		{"\n010lW#0200x3\r"},
		//Wrong response types (still detected by regex)
		{"\n010aW#020030\r"},
		{"\n010LW#020030\r"},
		{"\n010SW#020030\r"},
		{"\n010xW#020030\r"},
		{"\n010ZW#020030\r"},
		{"\n0109W#020030\r"},
		{"\n0105W#020030\r"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`Decode(%s)`, tc.input), func(t *testing.T) {
			_, err := encoder.Decode(tc.input)
			test.Error(t, err)
		})
	}
}

func TestDecodeResponseNoData(t *testing.T) {
	encoder, err := NewSerialEncoder()
	must.NoError(t, err)

	testCases := []struct {
		input string
	}{
		{"\n010lW#?\r"},
		{"\n001lW#?\r"},
		{"\n250lW#?\r"},
		{"\n010sW#?\r"},
		{"\n001sW#?\r"},
		{"\n250sW#?\r"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`Decode(%s)`, tc.input), func(t *testing.T) {
			_, err := encoder.Decode(tc.input)
			test.ErrorContains(t, err, "questionmark")
		})
	}
}
