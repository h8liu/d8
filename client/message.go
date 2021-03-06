package client

import (
	"net"
	"time"

	"github.com/h8liu/d8/packet"
	"github.com/h8liu/d8/printer"
)

type Message struct {
	RemoteAddr *net.UDPAddr
	Packet     *packet.Packet
	Timestamp  time.Time
}

func newMessage(q *Query, id uint16) *Message {
	return &Message{
		RemoteAddr: q.Server,
		Packet:     packet.Qid(q.Domain, q.Type, id),
		Timestamp:  time.Now(),
	}
}

func addrString(a *net.UDPAddr) string {
	if a.Port == 0 || a.Port == DNSPort {
		return a.IP.String()
	}
	return a.String()
}

func (self *Message) PrintTo(p printer.Interface) {
	p.Printf("@%s", addrString(self.RemoteAddr))
	self.Packet.PrintTo(p)
}

func (self *Message) String() string {
	return printer.String(self)
}
