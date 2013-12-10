package term

import (
	"d8/client"
)

type Leaf struct {
	Attempts []*client.Exchange
}

var _ Node = new(Leaf)

func (self *Leaf) IsLeaf() bool { return true }

func newLeaf(retry int) *Leaf {
	ret := new(Leaf)
	ret.Attempts = make([]*client.Exchange, 0, retry)
	return ret
}

func (self *Leaf) add(e *client.Exchange) {
	self.Attempts = append(self.Attempts, e)
}