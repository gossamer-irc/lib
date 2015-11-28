package lib

type EventHandler interface {
	OnServerLink(server *Server, hub *Server)
	OnChannelJoin(channel *Channel, client *Client, membership *Membership)
	OnChannelMessage(from *Client, to *Channel, message string)
	OnChannelModeChange(channel *Channel, by *Client, delta ChannelModeDelta, memberDelta []MemberModeDelta)
	OnPrivateMessage(from *Client, to *Client, message string)
}

type ProxyEventHandler struct {
	Delegate EventHandler
}

func (peh *ProxyEventHandler) OnServerLink(server *Server, hub *Server) {
	if peh.Delegate != nil {
		peh.Delegate.OnServerLink(server, hub)
	}
}

func (peh *ProxyEventHandler) OnPrivateMessage(from *Client, to *Client, message string) {
	if peh.Delegate != nil {
		peh.Delegate.OnPrivateMessage(from, to, message)
	}
}

func (peh *ProxyEventHandler) OnChannelJoin(channel *Channel, client *Client, membership *Membership) {
	if peh.Delegate != nil {
		peh.Delegate.OnChannelJoin(channel, client, membership)
	}
}

func (peh *ProxyEventHandler) OnChannelMessage(from *Client, to *Channel, message string) {
	if peh.Delegate != nil {
		peh.Delegate.OnChannelMessage(from, to, message)
	}
}

func (peh *ProxyEventHandler) OnChannelModeChange(channel *Channel, by *Client, delta ChannelModeDelta, memberDelta []MemberModeDelta) {
	if peh.Delegate != nil {
		peh.Delegate.OnChannelModeChange(channel, by, delta, memberDelta)
	}
}
