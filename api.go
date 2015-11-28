package lib

import (
	"io"
	"strings"
	"time"
)

func (n *Node) BeginLink(reader io.ReadCloser, writer io.WriteCloser, logger io.Writer) {
	// Set up the link itself.
	ch := make(chan LinkMessage)
	n.linkReadWg.Add(1)
	go func() {
		defer n.linkReadWg.Done()
		for msg := range ch {
			n.linkRecv <- msg
		}
	}()
	link := NewLink(reader, writer, 1024000, GobServerProtocolFactory, ch, n.wg)
	n.NewLinks[link] = newLink{link, logger}

	// Say hello.
	timestampMs := uint64(time.Now().UnixNano() / (1000 * 1000))
	link.WriteMessage(SSHello{1, timestampMs, n.config.ServerName, n.config.ServerDesc, n.DefaultSubnet.Name})
}

func (n *Node) JoinOrCreateChannel(client *Client, subnet *Subnet, name string) (*Channel, error) {
	lname := strings.ToLower(name)
	channel, found := subnet.Channel[lname]
	if !found {
		// Creating a new channel.
		channel := NewChannel(subnet, name)
		channel.Ts = time.Now().UTC()

		// Set mode +nt.
		channel.Mode.NoExternalMessages = true
		channel.Mode.TopicProtected = true

		// Create the first membership.
		mship := &Membership{
			Ts:      channel.Ts,
			IsOwner: true,
		}
		channel.LocalMember[client] = mship
		channel.Member[client] = mship
		subnet.Channel[lname] = channel

		n.SendAll(channel.Serialize())

		n.Handler.OnChannelJoin(channel, client, mship)
	} else {
		// Joining an existing channel.
		mship, found := channel.Member[client]
		if found {
			// Already an existing member.
			return nil, AlreadyAMemberError{}
		}

		// Here is where we would check bans, if they were implemented.

		mship = &Membership{
			Ts: time.Now().UTC(),
		}
		channel.LocalMember[client] = mship
		channel.Member[client] = mship

		n.SendAll(mship.Serialize(channel, client))

		n.Handler.OnChannelJoin(channel, client, mship)
	}
	return channel, nil
}

func (n *Node) ChannelMessage(client *Client, channel *Channel, message string) {
	n.Handler.OnChannelMessage(client, channel, message)
}

func (n *Node) ChangeChannelMode(client *Client, channel *Channel, channelModes ChannelModeDelta, memberModes []MemberModeDelta) {
	channelModes, memberModes = FilterChannelModes(channel, client, channelModes, memberModes)
	n.SendAll(SerializeChannelModeChange(channel, client, channelModes, memberModes))
	appliedDelta, appliedMembers := channel.ApplyModeDelta(channelModes, memberModes)
	if !appliedDelta.IsEmpty() || len(appliedMembers) > 0 {
		n.Handler.OnChannelModeChange(channel, client, appliedDelta, appliedMembers)
	}
}
