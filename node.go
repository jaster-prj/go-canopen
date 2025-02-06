package canopen

import "errors"

// Node is a canopen node
type Node struct {
	// Each node has an id, which is ArbitrationID & 0x7F
	ID int

	Network   *Network
	ObjectDic *DicObjectDic

	SDOClient *SDOClient
	PDONode   *PDONode
	NMTMaster *NMTMaster
}

func NewNode(id int, network *Network, objectDic *DicObjectDic) *Node {
	node := &Node{
		ID:        id,
		Network:   network,
		ObjectDic: objectDic,
	}

	return node
}

// GetId returns Node ID
func (node *Node) GetId() int {
	return node.ID
}

// FindName gets DicObject from ObjectDic
func (node *Node) FindName(name string) DicObject {
	if node.ObjectDic == nil {
		return nil
	}
	return node.ObjectDic.FindName(name)
}

// Send sends Frame with arbitration ID by connected network
func (node *Node) Send(arbID uint32, data []byte) error {
	if node.Network == nil {
		return errors.New("Network not defined")
	}
	return node.Network.Send(arbID, data)
}

// AcquireFramesChanFromNetwork gets new Channel for given FilterFunc
func (node *Node) AcquireFramesChanFromNetwork(filterFunc networkFramesChanFilterFunc) *NetworkFramesChan {
	if node.Network == nil {
		return nil
	}
	return node.Network.AcquireFramesChan(filterFunc)
}

// ReleaseFramesChanFromNetwork free channel with given id from network
func (node *Node) ReleaseFramesChanFromNetwork(id string) {
	if node.Network != nil {
		node.Network.ReleaseFramesChan(id)
	}
}

// SetNetwork set node.Network to the desired network
func (node *Node) SetNetwork(network *Network) {
	node.Network = network
}

// SetObjectDic set node.ObjectDic to the desired ObjectDic
func (node *Node) SetObjectDic(objectDic *DicObjectDic) {
	node.ObjectDic = objectDic
}

// Init create sdo clients, pdo nodes, nmt master
func (node *Node) Init() {
	node.SDOClient = NewSDOClient(node)
	node.PDONode = NewPDONode(node)
	node.NMTMaster = NewNMTMaster(node.ID, node.Network)

	// @TODO: list for NMTMaster
	// @TODO: implement EMCY
}

// Stop node
func (node *Node) Stop() {
	// Stop nmt master
	node.NMTMaster.UnlistenForHeartbeat()

	// Stop pdo listeners
	for _, mm := range node.PDONode.RX.Maps {
		mm.Unlisten()
	}
	for _, mm := range node.PDONode.TX.Maps {
		mm.Unlisten()
	}
}
