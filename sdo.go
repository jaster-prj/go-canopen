package canopen

import (
	"errors"
	"time"

	"github.com/jaster-prj/go-can"
)

const (
	SDORequestUpload    uint8 = 2 << 5
	SDOResponseUpload   uint8 = 2 << 5
	SDORequestDownload  uint8 = 1 << 5
	SDOResponseDownload uint8 = 3 << 5

	SDORequestSegmentUpload    uint8 = 3 << 5
	SDOResponseSegmentUpload   uint8 = 0 << 5
	SDORequestSegmentDownload  uint8 = 0 << 5
	SDOResponseSegmentDownload uint8 = 1 << 5

	SDOExpedited     uint8 = 0x2
	SDOSizeSpecified uint8 = 0x1
	SDOToggleBit     uint8 = 0x10
	SDONoMoreData    uint8 = 0x1
)

// SDOClient represent an SDO client
type SDOClient struct {
	Node      INode
	RXCobID   uint32
	TXCobID   uint32
	SendQueue []string
}

func NewSDOClient(node INode) *SDOClient {
	return &SDOClient{
		Node:      node,
		RXCobID:   uint32(0x600 + node.GetId()),
		TXCobID:   uint32(0x580 + node.GetId()),
		SendQueue: []string{},
	}
}

// SendRequest to network bus
func (sdoClient *SDOClient) SendRequest(req []byte) error {
	return sdoClient.Node.Send(sdoClient.RXCobID, req)
}

// FindName find an sdo object from object dictionary by name
func (sdoClient *SDOClient) FindName(name string) DicObject {
	if ob := sdoClient.Node.FindName(name); ob != nil {
		ob.SetSDO(sdoClient)
		return ob
	}

	return nil
}

// Send message and optionaly wait for response
func (sdoClient *SDOClient) Send(
	req []byte,
	expectFunc networkFramesChanFilterFunc,
	timeout *time.Duration,
	retryCount *int,
) (*can.Frame, error) {
	// If no response wanted, just send and return
	if expectFunc == nil {
		if err := sdoClient.SendRequest(req); err != nil {
			return nil, err
		}

		return nil, nil
	}

	// Set default timeout
	if timeout == nil {
		dtm := time.Duration(500) * time.Millisecond
		timeout = &dtm
	}

	if retryCount == nil {
		rtc := 4
		retryCount = &rtc
	}

	var expectSdoFunc networkFramesChanFilterFunc
	if expectFunc != nil {
		expectSdoFilterFunc := func(frm *can.Frame) bool {
			arbitrationId := frm.ArbitrationID
			if arbitrationId != sdoClient.TXCobID {
				return false
			}
			return (*expectFunc)(frm)
		}
		expectSdoFunc = &expectSdoFilterFunc
	}
	framesChan := sdoClient.Node.AcquireFramesChanFromNetwork(expectSdoFunc)
	defer sdoClient.Node.ReleaseFramesChanFromNetwork(framesChan.ID)

	// Retry loop
	remainingCount := *retryCount
	for {
		if remainingCount == 0 {
			break
		}

		if err := sdoClient.SendRequest(req); err != nil {
			return nil, err
		}

		timer := time.NewTimer(*timeout)

		loop := true
		for {
			if !loop {
				break
			}
			select {
			case <-timer.C:
				// Double timeout for each retry
				newTimeout := *timeout * 2
				timeout = &newTimeout
				loop = false
			case fr := <-framesChan.C:
				return fr, nil
			}
		}

		timer.Stop()
		remainingCount--
	}

	return nil, errors.New("timeout execeded")
}

// Read sdo
func (sdoClient *SDOClient) Read(index uint16, subIndex uint8) ([]byte, error) {
	reader := NewSDOReader(sdoClient, index, subIndex)
	return reader.ReadAll()
}

// Write sdo
func (sdoClient *SDOClient) Write(index uint16, subIndex uint8, forceSegment bool, data []byte) error {
	writer := NewSDOWriter(sdoClient, index, subIndex, forceSegment)
	return writer.Write(data)
}
