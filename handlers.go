package lib

import (
	"fmt"
	"log"
	"strings"
)

func (n *Node) handleLinkMessage(msg SSMessage, from *Server) {
	log.Printf("[%s <- %s]: %s", n.Me.Name, from.Name, msg.String())
	switch msg := msg.(type) {
	default:
		if msg == nil {
			log.Printf("Null message from [%s]", from.Name)
		} else {
			log.Printf("Unknown message from [%s]: %s", from.Name, msg.String())
		}
	case *SSBurstComplete:
		log.Printf("[%s] burst from %s complete", n.Me.Name, msg.Server)
		n.SendAllSkip(msg, from)
		n.bumpVersion()
	case *SSClient:
		n.handleClient(msg, from)
	case *SSServer:
		n.handleServer(msg, from)
	case *SSSync:
		n.handleSync(msg, from)
	case *SSKill:
		n.handleKill(msg, from)
	case *SSSplit:
		n.handleSplit(msg, from)
	case *SSChannel:
		n.handleChannel(msg, from)
	case *SSMembership:
		n.handleMembership(msg, from)
	case *SSPrivateMessage:
		n.handlePrivateMessage(msg, from)
	case *SSChannelMessage:
		n.handleChannelMessage(msg, from)
	case *SSChannelMode:
		n.handleChannelMode(msg, from)
	}
}

func (n *Node) handleNewLinkMessage(msg LinkMessage, nl newLink) {
	hello, ok := msg.msg.(*SSHello)
	if !ok {
		return
	}

	delete(n.NewLinks, msg.link)
	_, haveServer := n.Network[hello.Name]
	if haveServer {
		log.Printf("[%s] Already have server: %s", n.Me.Name, hello.Name)
		msg.link.Close()
		return
	}

	msg.link.SetName(fmt.Sprintf("%s <-> %s", n.Me.Name, hello.Name))

	server := NewLocalServer(hello.Name, hello.Description, msg.link, n.Me)
	log.Printf("[%s] got new local server %s", n.Me.Name, hello.Name)
	n.BurstTo(server)
	n.SendAll(server.Serialize())
	log.Printf("[%s] bursted %s", n.Me.Name, hello.Name)

	n.Local[msg.link] = server
	n.Network[server.Name] = server
	n.Me.Links[server.Name] = server

	n.Handler.OnServerLink(server, n.Me)
}

func (n *Node) handleChannel(msg *SSChannel, from *Server) {
	// Create a channel optimistically. It'll be thrown away if the channel exists locally.
	subnet, found := n.Subnet[msg.Subnet]
	if !found {
		log.Fatalf("[%s] on channel message [%s] unknown subnet: %s", n.Me.Name, msg.String(), msg.Subnet)
	}
	// Create a channel optimistically. It'll be thrown away if the channel exists locally.
	channel := NewChannel(subnet, msg.Name)
	channel.Ts = msg.Ts

	// Whether we trust remote and/or local modes is determined by the relative timestamps of the channels.
	// Identical timestamps means these are the same channel that was split and is being rejoined, and both
	// sides are acceptable. A newer timestamp on either side means the channel was re-created and should
	// not be trusted.
	trustRemote := true
	trustLocal := true
	existing, found := subnet.Channel[channel.Lname]
	if found {
		// Yep, there's a collision. Compare the two timestamps. The other server should make the opposite decision here.
		if existing.Ts.Before(channel.Ts) {
			// Local channel is older - don't trust remote channel's modes.
			trustRemote = false
		} else if channel.Ts.Before(existing.Ts) {
			// Local channel is newer - don't trust its modes.
			trustLocal = false
		}
		channel = existing
	} else {
		subnet.Channel[channel.Lname] = channel
	}

	// Keep a running list of member mode deltas to notify the handler later.
	deltas := make([]MemberModeDelta, 0)

	if !trustLocal {
		// Can't trust any of the local modes. Go through the existing channel and track all de-modes.
		for _, mship := range channel.Member {
			if mship.IsOwner || mship.IsAdmin || mship.IsOp || mship.IsHalfop || mship.IsVoice {
				delta := MemberModeDelta{}
				if mship.IsOwner {
					delta.IsOwner = MODE_REMOVED
					mship.IsOwner = false
				}
				if mship.IsAdmin {
					delta.IsAdmin = MODE_REMOVED
					mship.IsAdmin = false
				}
				if mship.IsOp {
					delta.IsOp = MODE_REMOVED
					mship.IsOp = false
				}
				if mship.IsHalfop {
					delta.IsHalfop = MODE_REMOVED
					mship.IsHalfop = false
				}
				if mship.IsVoice {
					delta.IsVoice = MODE_REMOVED
					mship.IsVoice = false
				}
				deltas = append(deltas, delta)
			}
		}
	}

	// Process new members.
	log.Printf("Processing new members...")
	for _, memMsg := range msg.Members {
		// Get the client by id. If it's not found, just ignore it.
		client, found := n.lookupClientById(memMsg.Client)
		if !found {
			continue
		}
		mship := &Membership{
			Ts: memMsg.Ts,
		}
		if trustRemote && (memMsg.IsOwner || memMsg.IsAdmin || memMsg.IsOp || memMsg.IsHalfop || memMsg.IsVoice) {
			delta := MemberModeDelta{
				Client: client,
			}
			if memMsg.IsOwner {
				mship.IsOwner = true
				delta.IsOwner = MODE_ADDED
			}
			if memMsg.IsAdmin {
				mship.IsAdmin = true
				delta.IsAdmin = MODE_ADDED
			}
			if memMsg.IsOp {
				mship.IsOp = true
				delta.IsOp = MODE_ADDED
			}
			if memMsg.IsHalfop {
				mship.IsHalfop = true
				delta.IsHalfop = MODE_ADDED
			}
			if memMsg.IsVoice {
				mship.IsVoice = true
				delta.IsVoice = MODE_ADDED
			}
			deltas = append(deltas, delta)
		}
		channel.Member[client] = mship
	}

	n.SendAllSkip(msg, from)

	// Notify mode changes, if any.
	if len(deltas) > 0 {
		n.Handler.OnChannelModeChange(channel, nil, ChannelModeDelta{}, deltas)
	}
}

func (n *Node) handleMembership(msg *SSMembership, from *Server) {
	client, found := n.lookupClientById(msg.Client)
	if !found {
		return
	}

	channel, found := n.lookupChannelById(msg.Channel)
	if !found {
		return
	}

	_, found = channel.Member[client]
	if found {
		return
	}

	mship := &Membership{
		Ts: msg.Ts,
	}

	delta := MemberModeDelta{Client: client}
	mode := false
	if msg.IsOwner {
		mship.IsOwner = true
		delta.IsOwner = MODE_ADDED
		mode = true
	}
	if msg.IsAdmin {
		mship.IsAdmin = true
		delta.IsAdmin = MODE_ADDED
		mode = true
	}
	if msg.IsOp {
		mship.IsOp = true
		delta.IsOp = MODE_ADDED
		mode = true
	}
	if msg.IsHalfop {
		mship.IsHalfop = true
		delta.IsHalfop = MODE_ADDED
		mode = true
	}
	if msg.IsVoice {
		mship.IsVoice = true
		delta.IsVoice = MODE_ADDED
		mode = true
	}

	channel.Member[client] = mship
	n.Handler.OnChannelJoin(channel, client, mship)
	if mode {
		n.Handler.OnChannelModeChange(channel, nil, ChannelModeDelta{}, []MemberModeDelta{delta})
	}
	n.SendAllSkip(msg, from)
}

func (n *Node) handleClient(msg *SSClient, from *Server) {
	client := &Client{
		Nick:   msg.Nick,
		Lnick:  strings.ToLower(msg.Nick),
		Ident:  msg.Ident,
		Vident: msg.Vident,
		Host:   msg.Host,
		Vhost:  msg.Vhost,
		Ip:     msg.Ip,
		Vip:    msg.Vip,
		Gecos:  msg.Gecos,
		Ts:     msg.Ts,
	}
	server, found := n.Network[msg.Server]
	if !found {
		log.Fatalf("[%s] on client message [%s] unknown server: %s", n.Me.Name, msg.String(), msg.Server)
	}
	client.Server = server

	subnet, found := n.Subnet[msg.Subnet]
	if !found {
		log.Fatalf("[%s] on client message [%s] unknown subnet: %s", n.Me.Name, msg.String(), msg.Subnet)
	}
	client.Subnet = subnet

	existing, found := subnet.Client[client.Lnick]
	if found {
		// Collision! Either one client is younger and must die, or they are the same
		// age exactly, and must both die.

		if !existing.Ts.Before(client.Ts) {
			kill := &SSKill{
				Id:         existing.Id(),
				Server:     n.Me.Name,
				Authority:  false,
				Reason:     "Nickname collision (older)",
				ReasonCode: SS_KILL_REASON_COLLISION,
			}
			if existing.Server == n.Me {
				kill.Authority = true
				n.processQuit(existing, kill.Reason)
				n.SendAllSkip(kill, from)
			} else {
				existing.Server.Send(kill)
			}
		}

		// Skip propagation if the incoming client shouldn't survive
		// either.
		//
		// When this happens, we could still receive messages concerning this client.
		// They should be discarded since the Id of the client will be incorrect (wrong
		// server).
		if !client.Ts.Before(existing.Ts) {
			log.Printf("[%s] not adding client [%s] - collision, too young", n.Me.Name, client.DebugString())
			return
		}
	}

	subnet.Client[client.Lnick] = client
	log.Printf("[%s] added client %s", n.Me.Name, client.DebugString())

	n.SendAllSkip(msg, from)
}

func (n *Node) handleServer(msg *SSServer, from *Server) {
	_, found := n.Network[msg.Name]
	if found {
		log.Fatalf("[%s] already linked: %s", n.Me.Name, msg.Name)
	}
	via, found := n.Network[msg.Via]
	if !found {
		log.Fatalf("[%s] %s via %s but that doesn't exist", n.Me.Name, msg.Name, msg.Via)
	}

	server := NewRemoteServer(msg.Name, msg.Desc, via)
	n.Network[msg.Name] = server
	log.Printf("[%s] attaching %s via %s", n.Me.Name, server.Name, server.Hub.Name)
	n.SendAllSkip(msg, from)
	n.Handler.OnServerLink(server, via)
}

func (n *Node) handleSync(msg *SSSync, from *Server) {
	if !msg.Reply {
		// We need to reply.

		reply := &SSSync{
			Sequence:  msg.Sequence,
			Reply:     true,
			Origin:    msg.Origin,
			ReplyFrom: n.config.ServerName,
		}

		// Reply goes back along the route from whence it came. From now on
		// it will be routed by its Origin.
		from.Send(reply)

		// And pass on the Sync message to the next couple nodes.
		n.SendAllSkip(msg, from)
	} else {
		origin, found := n.Network[msg.Origin]
		if !found {
			log.Fatalf("[sync] unknown origin: %s", msg.Origin)
		}
		if origin.Route == from {
			log.Fatalf("[sync] loop detected: %s", msg.Origin)
		}

		if origin != n.Me {
			origin.Send(msg)
			return
		}

		sr, found := n.syncsActive[msg.Sequence]
		if !found {
			log.Fatalf("[sync] unknown sequence: %d", msg.Sequence)
		}
		sr.servers[msg.ReplyFrom] = true

		for _, flag := range sr.servers {
			if !flag {
				return
			}
		}

		// Completely synced.
		delete(n.syncsActive, msg.Sequence)
		sr.synced <- struct{}{}
	}
}

func (n *Node) handleKill(msg *SSKill, from *Server) {
	client, found := n.lookupClientById(msg.Id)
	if msg.Authority {
		// This is a kill order!
		client, found := n.lookupClientById(msg.Id)
		if found {
			n.processQuit(client, msg.Reason)
		}

		n.SendAllSkip(msg, from)
	} else {
		if !found {
			return
		}
		if client.Server == n.Me {
			// Instruction to kill the client.
			msg.Authority = true
			n.SendAll(msg)
		} else if client.Server.Route != from {
			client.Server.Send(msg)
		}
	}
}

func (n *Node) handleSplit(msg *SSSplit, from *Server) {
	server, found := n.Network[msg.Server]
	if !found {
		log.Fatalf("[%s] split of %s but not connected", n.Me.Name, msg.Server)
	}
	if server.Route != from {
		log.Fatalf("[%s] split of server %s from wrong direction!", n.Me.Name, msg.Server)
	}

	n.processSplit(server, msg.Reason)
	n.SendAllSkip(msg, from)
	n.bumpVersion()
}

func (n *Node) handlePrivateMessage(msg *SSPrivateMessage, from *Server) {
	to, found := n.lookupClientById(msg.To)
	if !found {
		log.Printf("PM to unknown user: %s", msg.To)
		return
	}

	if to.IsLocal() {
		from, found := n.lookupClientById(msg.From)
		if !found {
			log.Printf("PM from unknown user: %s", msg.From)
			return
		}
		n.Handler.OnPrivateMessage(from, to, msg.Message)
	} else {
		if to.Server.Route == from {
			// TODO disconnect server for being stupid
			log.Printf("Loop!")
			return
		}
		to.Server.Route.Send(msg)
	}
}

func (n *Node) handleChannelMessage(msg *SSChannelMessage, from *Server) {
	to, found := n.lookupChannelById(msg.To)
	if !found {
		log.Printf("CM to unknown channel: %s", msg.To)
		return
	}

	fromClient, found := n.lookupClientById(msg.From)
	if !found {
		log.Printf("CM from unknown user: %s", msg.From)
		return
	}

	n.Handler.OnChannelMessage(fromClient, to, msg.Message)
	n.SendAllSkip(msg, from)
}

func (n *Node) handleChannelMode(msg *SSChannelMode, from *Server) {
	target, found := n.lookupChannelById(msg.Channel)
	if !found {
		log.Printf("ChannelMode change on unknown channel: %s", msg.Channel)
		return
	}

	actor, found := n.lookupClientById(msg.From)
	if !found {
		log.Printf("ChannelMode change from unknown client: %s", msg.From)
	}

	memberDeltas := make([]MemberModeDelta, 0, len(msg.MemberMode))
	for _, delta := range msg.MemberMode {
		memberDelta, ok := delta.ToMemberModeDelta(n)
		if !ok {
			continue
		}
		memberDeltas = append(memberDeltas, memberDelta)
	}
	appliedMode, appliedMember := target.ApplyModeDelta(msg.Mode.ToChannelModeDelta(), memberDeltas)

	n.Handler.OnChannelModeChange(target, actor, appliedMode, appliedMember)
	n.SendAllSkip(msg, from)
}
