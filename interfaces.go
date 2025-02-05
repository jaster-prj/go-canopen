package canopen

import (
	"time"

	"github.com/angelodlfrtr/go-can"
)

type ISDOClient interface {
	FindName(name string) DicObject
	Read(index uint16, subIndex uint8) ([]byte, error)
	Send(req []byte, expectFunc networkFramesChanFilterFunc, timeout *time.Duration, retryCount *int) (*can.Frame, error)
	SendRequest(req []byte) error
	Write(index uint16, subIndex uint8, forceSegment bool, data []byte) error
}

type INode interface {
	GetId() int
	FindName(name string) DicObject
	Send(arbID uint32, data []byte) error
	AcquireFramesChanFromNetwork(filterFunc networkFramesChanFilterFunc) *NetworkFramesChan
	ReleaseFramesChanFromNetwork(id string)
}
