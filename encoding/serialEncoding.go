// encoding contains the code to encode and decode the data transmitted over the serial interface.
// It also contains the defintion of a frame (a single request or response send over the serial interface).
package encoding

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/ansel1/merry/v2"
	log "github.com/sirupsen/logrus"
)

const (
	CHAR_LF       = '\n'
	CHAR_CR       = '\r'
	CHAR_WRG      = 'W'
	CHAR_WRITE    = 's'
	CHAR_READ     = 'l'
	CHAR_RESPONSE = '#'
)

const (
	CHAR_S_LF       = string(CHAR_LF)
	CHAR_S_CR       = string(CHAR_CR)
	CHAR_S_WRG      = string(CHAR_WRG)
	CHAR_S_WRITE    = string(CHAR_WRITE)
	CHAR_S_READ     = string(CHAR_READ)
	CHAR_S_RESPONSE = string(CHAR_RESPONSE)
)

const (
	// TEMPLATE_WRITE is the template for creating a write frame
	TEMPLATE_WRITE = CHAR_S_LF + "{{printf \"%03d\" .Address}}" + CHAR_S_WRITE + CHAR_S_WRG + "{{printf \"%03d\" .Function}}{{printf \"%03d\" .Value}}" + CHAR_S_CR
	// TEMPLATE_READ is the template for creating a read frame
	TEMPLATE_READ = CHAR_S_LF + "{{printf \"%03d\" .Address}}" + CHAR_S_READ + CHAR_S_WRG + "{{printf \"%03d\" .Function}}" + CHAR_S_CR
)

const (
	REGEX_3DIGIT_NUM = "([0-9]{3})"
	REGEX_READ_WRITE = "(" + CHAR_S_READ + "|" + CHAR_S_WRITE + ")"
	// REGEX_RESPONSE is the regex that machtes a response frame
	REGEX_RESPONSE = CHAR_S_LF + REGEX_3DIGIT_NUM + REGEX_READ_WRITE + CHAR_S_WRG + CHAR_S_RESPONSE + "(" + REGEX_3DIGIT_NUM + REGEX_3DIGIT_NUM + "|\\?)" + CHAR_S_CR
)

// A SerialEncoder can be used to encode and decode frames to and from their string representation
type SerialEncoder struct {
	writeTemplate *template.Template
	readTemplate  *template.Template
	responseRegex *regexp.Regexp
}

func (serialEncoder *SerialEncoder) buildTemplates() error {
	tmpl, err := template.New("WriteFrame").Parse(TEMPLATE_WRITE)
	if err != nil {
		return merry.Prepend(err, "Failed building WriteFrame template")
	}
	serialEncoder.writeTemplate = tmpl

	tmpl, err = template.New("ReadFrame").Parse(TEMPLATE_READ)
	if err != nil {
		return merry.Prepend(err, "Failed building ReadFrame template")
	}
	serialEncoder.readTemplate = tmpl

	return nil
}

func (serialEncoder *SerialEncoder) buildRegex() error {
	re, err := regexp.Compile(REGEX_RESPONSE)
	if err != nil {
		return merry.Prepend(err, "Failed building Response regex")
	}
	serialEncoder.responseRegex = re

	return nil
}

// NewSerialEncoder initializes and returns a new SerialEncoder
func NewSerialEncoder() (*SerialEncoder, error) {
	serialEncoder := &SerialEncoder{}
	if err := serialEncoder.buildTemplates(); err != nil {
		return nil, err
	}
	if err := serialEncoder.buildRegex(); err != nil {
		return nil, err
	}
	return serialEncoder, nil
}

// Encode encodes the given frame into its string representation
func (serialEncoder SerialEncoder) Encode(frame Frame) (string, error) {
	var buf bytes.Buffer

	log.WithField("frame", frame).Trace("Encoding frame")

	if frame.FrameType() == ReadRequest {
		if err := serialEncoder.readTemplate.Execute(&buf, frame); err != nil {
			return "", merry.Prepend(err, "Failed executing ReadFrame template")
		}
	} else if frame.FrameType() == WriteRequest {
		if err := serialEncoder.writeTemplate.Execute(&buf, frame); err != nil {
			return "", merry.Prepend(err, "Failed executing WriteFrame template")
		}
	} else {
		return "", merry.Errorf("Can't encode a frame of type %d", frame.FrameType())
	}

	data := buf.String()

	log.WithField("data", DataWithEscapeChars(data)).Trace("Encoded frame")

	return data, nil
}

func parseUint16(data string) (uint16, error) {
	value, err := strconv.ParseUint(data, 10, 16)
	return uint16(value), merry.Prependf(err, "Failed to decode string into uint16: %s", data)
}

// Decode decodes the given frame from its string representation
func (serialEncoder SerialEncoder) Decode(data string) (Frame, error) {
	log.WithField("data", DataWithEscapeChars(data)).Trace("Decoding frame")

	strings := serialEncoder.responseRegex.FindStringSubmatch(data)
	if strings == nil {
		return nil, merry.Errorf("Unable to decode the following data: %s", data)
	}

	stringsToPrint := make([]string, len(strings))
	for i, s := range strings {
		stringsToPrint[i] = DataWithEscapeChars(s)
	}

	if strings[3] == "?" {
		return nil, merry.Errorf("Device returned frame with qustionmark instead of data. Was the function valid?")
	}

	address, err := parseUint16(strings[1])
	if err != nil {
		return nil, err
	}
	function, err := parseUint16(strings[4])
	if err != nil {
		return nil, err
	}
	value, err := parseUint16(strings[5])
	if err != nil {
		return nil, err
	}

	var frameType FrameType
	if strings[2] == CHAR_S_READ {
		frameType = ReadResponse
	} else if strings[2] == CHAR_S_WRITE {
		frameType = WriteResponse
	} else {
		return nil, merry.Errorf("Unknown frame response type: %s", strings[2])
	}

	frame, err := newReponse(frameType, address, function, value)

	if err == nil {
		log.WithField("frame", frame).Trace("Succsesfully decoded frame")
	} else {
		log.WithField("frame", frame).WithError(err).Debug("Error decoding frame")
	}

	return frame, err
}

func DataWithEscapeChars(data string) string {
	return strings.ReplaceAll(strings.ReplaceAll(data, "\n", "\\n"), "\r", "\\r")
}
