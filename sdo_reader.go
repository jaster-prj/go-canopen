package canopen

import (
	"encoding/binary"
	"errors"

	"github.com/jaster-prj/go-can"
)

type SDOReader struct {
	SDOClient *SDOClient
	Index     uint16
	SubIndex  uint8
	Toggle    uint8
	Pos       int
	Size      uint32
	Data      []byte
}

func NewSDOReader(sdoClient *SDOClient, index uint16, subIndex uint8) *SDOReader {
	return &SDOReader{
		SDOClient: sdoClient,
		Index:     index,
		SubIndex:  subIndex,
		Data:      []byte{},
	}
}

// buildRequestUploadBuf working
func (reader *SDOReader) buildRequestUploadBuf() []byte {
	buf := make([]byte, 8) // 8 len is important

	buf[0] = SDORequestUpload
	binary.LittleEndian.PutUint16(buf[1:], reader.Index)
	buf[3] = reader.SubIndex

	return buf
}

// buildRequestSegmentUploadBuf
func (reader *SDOReader) buildRequestSegmentUploadBuf() []byte {
	buf := make([]byte, 8)

	command := SDORequestSegmentUpload
	command |= reader.Toggle
	buf[0] = command

	return buf
}

// RequestUpload returns data if EXPEDITED, else nil
func (reader *SDOReader) RequestUpload() ([]byte, error) {
	expectFunc := func(frm *can.Frame) bool {
		resCommand := frm.Data[0]
		resIndex := binary.LittleEndian.Uint16(frm.Data[1:])
		resSubindex := frm.Data[3]

		// Check response validity
		if (resCommand & 0xE0) != SDOResponseUpload {
			return false
		}

		if resIndex != reader.Index {
			return false
		}

		if resSubindex != reader.SubIndex {
			return false
		}

		return true
	}

	frm, err := reader.SDOClient.Send(reader.buildRequestUploadBuf(), &expectFunc, nil, nil)
	if err != nil {
		return nil, err
	}

	resCommand := frm.Data[0]
	resData := frm.Data[4:8]

	var expData []byte

	// If data is already in response (max 4 bytes)
	if (resCommand & SDOExpedited) != 0 {
		// Expedited upload
		if (resCommand & SDOSizeSpecified) != 0 {
			reader.Size = uint32(4 - ((resCommand >> 2) & 0x3))
			expData = resData[0:reader.Size]
		} else {
			expData = resData
		}

		return expData, nil
	}

	if (resCommand & SDOSizeSpecified) != 0 {
		reader.Size = binary.LittleEndian.Uint32(resData[0:])
	}

	// Will have to use segmented upload
	return nil, nil
}

// Read segmented uploads
func (reader *SDOReader) Read() (*can.Frame, error) {
	expectFunc := func(frm *can.Frame) bool {
		if frm == nil {
			return false
		}

		if frm.ArbitrationID != reader.SDOClient.TXCobID {
			return false
		}

		resCommand := frm.Data[0]
		return (resCommand & 0xE0) == SDOResponseSegmentUpload
	}

	return reader.SDOClient.Send(reader.buildRequestSegmentUploadBuf(), &expectFunc, nil, nil)
}

// ReadAll ..
func (reader *SDOReader) ReadAll() ([]byte, error) {
	data, err := reader.RequestUpload()
	if err != nil {
		return nil, err
	}

	// If EXPEDITED, return data
	if data != nil {
		return data, nil
	}

	// Use Segmented upload
	for {
		frm, err := reader.Read()
		if err != nil {
			return nil, err
		}

		resCommand := frm.Data[0]
		if (resCommand & SDOToggleBit) != reader.Toggle {
			return nil, errors.New("toggle bit mismatch")
		}

		length := int(7 - ((resCommand >> 1) & 0x7))
		reader.Toggle ^= SDOToggleBit
		reader.Pos += length

		// Append data
		reader.Data = append(reader.Data, frm.Data[1:length+1]...)

		// If no more data
		if (resCommand & SDONoMoreData) != 0 {
			break
		}

		// Continue, read next segment
	}

	return reader.Data, nil
}
