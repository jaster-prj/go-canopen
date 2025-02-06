package canopen

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jaster-prj/go-can"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	expectFuncSDOWithoutFilter = func(frm *can.Frame) bool {
		return true
	}
	expectFuncSDOMissingFirst = func(frm *can.Frame) bool {
		return (frm.Data[0] & 0xE0) == byte(0x60)
	}
)

type send_response struct {
	wait  time.Duration
	frame can.Frame
}

type networkMock struct {
	networkFramesChans []*NetworkFramesChan
}

func (n *networkMock) AcquireFramesChan(filterFunc networkFramesChanFilterFunc) *NetworkFramesChan {
	chanID := uuid.Must(uuid.NewRandom()).String()
	frameChan := &NetworkFramesChan{
		ID:     chanID,
		Filter: filterFunc,
		C:      make(chan *can.Frame),
	}

	// Append network.FramesChans
	n.networkFramesChans = append(n.networkFramesChans, frameChan)

	return frameChan
}

func (n *networkMock) ReleaseFramesChan(id string) {

	var framesChan *NetworkFramesChan
	var framesChanIndex *int

	for idx, fc := range n.networkFramesChans {
		if fc.ID == id {
			framesChan = fc
			idxx := idx
			framesChanIndex = &idxx
			break
		}
	}

	if framesChanIndex == nil {
		return
	}

	// Close chan
	close(framesChan.C)

	// Remove frameChan from network.FramesChans
	n.networkFramesChans = append(
		n.networkFramesChans[:*framesChanIndex],
		n.networkFramesChans[*framesChanIndex+1:]...,
	)
}

func (n *networkMock) ReceiveFrame(frame *can.Frame) {
	for _, ch := range n.networkFramesChans {
		ch.Publish(frame)
	}
}

type nodeMock struct {
	mock.Mock

	id      int
	network networkMock
}

func (n *nodeMock) GetId() int {
	return n.id
}

func (n *nodeMock) FindName(name string) DicObject {
	args := n.Called(name)
	return args.Get(0).(DicObject)
}

func (n *nodeMock) Send(arbID uint32, data []byte) error {
	args := n.Called(arbID, data)
	sendResponseList := args.Get(1).([]send_response)
	go func(sendResponseListInternal []send_response) {
		for _, response := range sendResponseListInternal {
			time.Sleep(response.wait)
			n.network.ReceiveFrame(&response.frame)
		}
	}(sendResponseList)
	return args.Error(0)
}

func (n *nodeMock) AcquireFramesChanFromNetwork(filterFunc networkFramesChanFilterFunc) *NetworkFramesChan {
	return n.network.AcquireFramesChan(filterFunc)
}

func (n *nodeMock) ReleaseFramesChanFromNetwork(id string) {
	n.network.ReleaseFramesChan(id)
}

func getNodeWithoutResponse() INode {
	node := &nodeMock{
		id:      0,
		network: networkMock{},
	}
	node.On("Send", uint32(0x600), []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(nil, []send_response{})
	return node
}

func getNodeWithImmediatelyResponse() INode {
	node := &nodeMock{
		id:      0,
		network: networkMock{},
	}

	frame1 := can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	node.On("Send", uint32(0x600), []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(nil, []send_response{{wait: time.Millisecond, frame: frame1}})
	return node
}

func getNodeWithWrongArbitration() INode {
	node := &nodeMock{
		id:      0,
		network: networkMock{},
	}

	frame1 := can.Frame{ArbitrationID: 0x581, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	node.On("Send", uint32(0x600), []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(nil, []send_response{{wait: time.Millisecond * 100, frame: frame1}})
	return node
}

func getNodeWithRightArbitrationOnSecondFrame() INode {
	node := &nodeMock{
		id:      0,
		network: networkMock{},
	}

	frame1 := can.Frame{ArbitrationID: 0x581, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	frame2 := can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	node.On("Send", uint32(0x600), []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(nil, []send_response{{wait: time.Millisecond * 100, frame: frame1}, {wait: time.Millisecond * 100, frame: frame2}})
	return node
}

func getNodeWithArbitrationMissingFirst() INode {
	node := &nodeMock{
		id:      0,
		network: networkMock{},
	}

	frame1 := can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	frame2 := can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	node.On("Send", uint32(0x600), []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(nil, []send_response{{wait: time.Millisecond * 100, frame: frame1}, {wait: time.Millisecond * 100, frame: frame2}})
	return node
}

func TestSDOClient_Send(t *testing.T) {
	type args struct {
		req        []byte
		expectFunc networkFramesChanFilterFunc
		timeout    *time.Duration
		retryCount *int
	}
	tests := []struct {
		name    string
		getNode func() INode
		args    args
		want    *can.Frame
		wantErr bool
	}{
		{
			name:    "SDO get Frame without Response",
			getNode: getNodeWithoutResponse,
			args: args{
				req:        []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65},
				expectFunc: nil,
				timeout:    nil,
				retryCount: nil,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "SDO get Frame immediately",
			getNode: getNodeWithImmediatelyResponse,
			args: args{
				req:        []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65},
				expectFunc: &expectFuncSDOWithoutFilter,
				timeout:    nil,
				retryCount: nil,
			},
			want:    &can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}},
			wantErr: false,
		},
		{
			name:    "SDO get wrong arbitration",
			getNode: getNodeWithWrongArbitration,
			args: args{
				req:        []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65},
				expectFunc: &expectFuncSDOWithoutFilter,
				timeout:    nil,
				retryCount: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "SDO get right arbitration on second Frame",
			getNode: getNodeWithRightArbitrationOnSecondFrame,
			args: args{
				req:        []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65},
				expectFunc: &expectFuncSDOWithoutFilter,
				timeout:    nil,
				retryCount: nil,
			},
			want:    &can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}},
			wantErr: false,
		},
		{
			name:    "SDO get right Frame missing first",
			getNode: getNodeWithArbitrationMissingFirst,
			args: args{
				req:        []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65},
				expectFunc: &expectFuncSDOMissingFirst,
				timeout:    nil,
				retryCount: nil,
			},
			want:    &can.Frame{ArbitrationID: 0x580, Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdoClient := NewSDOClient(tt.getNode())
			got, err := sdoClient.Send(tt.args.req, tt.args.expectFunc, tt.args.timeout, tt.args.retryCount)
			if (err != nil) != tt.wantErr {
				t.Errorf("SDOClient.Send() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, tt.want, got) {
				t.Errorf("SDOClient.Send() = %v, want %v", got, tt.want)
			}
		})
	}
}
