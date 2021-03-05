package machinist

type Node struct {
	r *NodeRequest
}

type NodeRequest struct {
	Token string
}

func NewNodeRequest() *NodeRequest  {
	return &NodeRequest{}
}

//WithToken will specify a startup token
func (nr *NodeRequest) WithToken(t string) *NodeRequest {
	nr.Token = t
	return nr
}

func NewNode(req *NodeRequest) *Node{
	return &Node{
		r : req,
	}
}

