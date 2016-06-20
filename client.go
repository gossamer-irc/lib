package lib

import (
	"fmt"
	"time"
)

type Client struct {
	Subnet        *Subnet
	Server        *Server
	Nick, Lnick   string
	Ident, Vident string
	Host, Vhost   string
	Ip, Vip       string
	Gecos         string
	Ts            time.Time
	Member        map[*Channel]*Membership
}

func (c *Client) Id() SSClientId {
	return SSClientId{c.Server.Name, c.Subnet.Name, c.Lnick}
}

func (c *Client) IsLocal() bool {
	return c.Server.Hub == nil
}

func (c *Client) Serialize() *SSClient {
	return &SSClient{
		Subnet: c.Subnet.Name,
		Server: c.Server.Name,
		Nick:   c.Nick,
		Ident:  c.Ident,
		Vident: c.Vident,
		Host:   c.Host,
		Vhost:  c.Vhost,
		Ip:     c.Ip,
		Vip:    c.Vip,
		Gecos:  c.Gecos,
		Ts:     c.Ts,
	}
}

func (c *Client) DebugString() string {
	snName := "*"
	svName := "*"
	if c.Subnet != nil {
		snName = c.Subnet.Name
	}
	if c.Server != nil {
		svName = c.Server.Name
	}
	return fmt.Sprintf("client(server(%s) id(%s:%s!%s@%s) ip(%s) v(%s@%s) vip(%s) gecos(%s) ts(%v))", svName, snName, c.Nick, c.Ident, c.Host, c.Ip, c.Vident, c.Vhost, c.Vip, c.Gecos, c.Ts)
}
