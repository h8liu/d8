package tasks

import (
	. "d8/domain"
	pa "d8/packet"
	"d8/packet/consts"
	"d8/packet/rdata"
	. "d8/term"
)

type IPs struct {
	Domain     *Domain
	StartWith  *ZoneServers
	HeadLess   bool
	HideResult bool

	// inherit from the initializing Recur Task
	Return  int
	Packet  *pa.Packet
	EndWith *ZoneServers
	Zones   []*ZoneServers

	CnameTraceBack map[string]*Domain // in and out, inherit from father IPs

	CnameEndpoints []*Domain       // new endpoint cnames discovered
	CnameIPs       map[string]*IPs // sub IPs for each unresolved end point

	CnameRecords []*pa.RR // new cname records
	Records      []*pa.RR // new end point ip records
}

func NewIPs(d *Domain) *IPs {
	return &IPs{Domain: d}
}

// Look for Query error or A records in Answer
func (self *IPs) findResults(recur *Recur) bool {
	if recur.Return != Okay {
		return true
	}

	for _, rr := range recur.Answers {
		switch rr.Type {
		case consts.A:
			self.Records = append(self.Records, rr)
		case consts.CNAME:
			// okay
		default:
			panic("bug")
		}
	}

	return len(self.Records) > 0
}

func (self *IPs) findCnameResults(recur *Recur) (unresolved []*Domain) {
	unresolved = make([]*Domain, 0, len(self.CnameEndpoints))

	for _, cname := range self.CnameEndpoints {
		rrs := recur.Packet.SelectRecords(cname, consts.A)
		if len(rrs) == 0 {
			unresolved = append(unresolved, cname)
			continue
		}
		self.Records = append(self.Records, rrs...)
	}

	return
}

// Returns true when if finds any endpoints
func (self *IPs) extractCnames(recur *Recur, d *Domain, c Cursor) bool {
	if _, found := self.CnameTraceBack[d.String()]; !found {
		panic("bug")
	}

	if !self.EndWith.Serves(d) {
		// domain not in the zone
		// so even there were cname records about this domain
		// they cannot be trusted
		return false
	}

	rrs := recur.Packet.SelectRecords(d, consts.CNAME)
	ret := false

	for _, rr := range rrs {
		cname := rdata.ToDomain(rr.Rdata)
		cnameStr := cname.String()
		if self.CnameTraceBack[cnameStr] != nil {
			// some error cnames, pointing to self or forming circles
			continue
		}

		c.Printf("// cname: %v -> %v", d, cname)
		self.CnameRecords = append(self.CnameRecords, rr)
		self.CnameTraceBack[cname.String()] = d

		// see if it follows another CNAME
		if self.extractCnames(recur, cname, c) {
			// see so, then we only tracks the end point
			ret = true // we added an endpoint in the recursion
			continue
		}

		c.Printf("// cname endpoint: %v", cname)
		// these are end points that needs to be crawled
		self.CnameEndpoints = append(self.CnameEndpoints, cname)
		ret = true
	}

	return ret
}

func (self *IPs) PrintResult(c Cursor) {
	cnames, results := self.Results()

	for _, r := range cnames {
		c.Printf("// %v -> %v", r.Domain, rdata.ToDomain(r.Rdata))
	}
	for _, r := range results {
		c.Printf("// %v(%v)", r.Domain, rdata.ToIPv4(r.Rdata))
	}
}

func (self *IPs) Run(c Cursor) {
	if !self.HeadLess {
		c.Printf("ips %v {", self.Domain)
		c.ShiftIn()
		defer ShiftOutWith(c, "}")
	}

	self.run(c)
	if c.Error() != nil {
		return
	}

	if !self.HideResult {
		self.PrintResult(c)
	}
}

func (self *IPs) Results() (cnames, results []*pa.RR) {
	cnames = make([]*pa.RR, 0, 20)
	results = make([]*pa.RR, 0, 20)

	return self.results(cnames, results)
}

func (self *IPs) results(cnames, results []*pa.RR) (c, r []*pa.RR) {
	cnames = append(cnames, self.CnameRecords...)
	results = append(results, self.Records...)

	for _, ips := range self.CnameIPs {
		cnames, results = ips.results(cnames, results)
	}

	return cnames, results
}

func (self *IPs) run(c Cursor) {
	recur := NewRecur(self.Domain)
	recur.HeadLess = true
	recur.StartWith = self.StartWith

	_, e := c.T(recur)
	if e != nil {
		return
	}

	// inherit from recur
	self.Return = recur.Return
	self.EndWith = recur.EndWith
	self.Packet = recur.Packet
	self.Zones = recur.Zones

	self.Records = make([]*pa.RR, 0, 10)
	self.findResults(recur)

	// even if we find results, we still track cnames if any
	self.CnameEndpoints = make([]*Domain, 0, 10)
	if self.CnameTraceBack == nil {
		self.CnameTraceBack = make(map[string]*Domain)
		self.CnameTraceBack[self.Domain.String()] = nil
	} else {
		_, found := self.CnameTraceBack[self.Domain.String()]
		if !found {
			panic("bug")
		}
	}

	self.CnameRecords = make([]*pa.RR, 0, 10)
	if !self.extractCnames(recur, self.Domain, c) {
		return
	}

	if len(self.CnameEndpoints) == 0 {
		panic("bug")
	}

	unresolved := self.findCnameResults(recur)
	if len(unresolved) == 0 {
		return
	}

	// trace down the cnames
	p := self.Packet
	z := self.EndWith
	self.CnameIPs = make(map[string]*IPs)

	for _, cname := range unresolved {
		// search for redirects
		servers := ExtractServers(p, z.Zone(), cname, c)

		// check for last result
		if servers == nil {
			if z.Serves(cname) {
				servers = z
			}
		}

		if servers == nil {
			if self.StartWith != nil && self.StartWith.Serves(cname) {
				servers = self.StartWith
			}
		}

		cnameIPs := NewIPs(cname)
		cnameIPs.HideResult = true
		cnameIPs.StartWith = servers
		cnameIPs.CnameTraceBack = self.CnameTraceBack

		self.CnameIPs[cname.String()] = cnameIPs

		_, e := c.T(cnameIPs)
		if e != nil {
			return
		}
	}
}
