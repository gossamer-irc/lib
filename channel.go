package lib

import (
	"fmt"
	"strings"
	"time"
)

type ModeDelta uint8

func (delta ModeDelta) String() string {
	switch delta {
	case MODE_ADDED:
		return "+"
	case MODE_REMOVED:
		return "-"
	default:
		return "."
	}
}

const (
	MODE_UNCHANGED ModeDelta = iota
	MODE_ADDED
	MODE_REMOVED
)

type Channel struct {
	Node    *Node
	Subnet  *Subnet
	Name    string
	Lname   string
	Ts      time.Time
	Topic   string
	TopicTs time.Time
	TopicBy string

	LocalMember map[*Client]*Membership
	Member      map[*Client]*Membership

	Mode struct {
		TopicProtected     bool
		NoExternalMessages bool
		Moderated          bool
		Secret             bool
		Limit              uint32
		Key                string
	}
}

func NewChannel(node *Node, subnet *Subnet, name string) *Channel {
	return &Channel{
		Node:        node,
		Subnet:      subnet,
		Name:        name,
		Lname:       strings.ToLower(name),
		LocalMember: make(map[*Client]*Membership),
		Member:      make(map[*Client]*Membership),
	}
}

func (ch *Channel) Id() SSChannelId {
	return SSChannelId{
		Subnet: ch.Subnet.Name,
		Name:   ch.Name,
	}
}

func (ch *Channel) DebugString() string {
	return fmt.Sprintf("channel(%s, %s, %s)", ch.Subnet.Name, ch.Name, ch.Ts)
}

func (ch *Channel) Serialize() *SSChannel {
	msg := &SSChannel{
		Name:    ch.Name,
		Subnet:  ch.Subnet.Name,
		Ts:      ch.Ts,
		Members: make([]*SSMembership, 0, len(ch.Member)),
	}
	for client, member := range ch.Member {
		msg.Members = append(msg.Members, member.Serialize(ch, client))
	}
	return msg
}

func (ch *Channel) ApplyModeDelta(delta ChannelModeDelta, memberDelta []MemberModeDelta) (ChannelModeDelta, []MemberModeDelta) {
	outDelta := ChannelModeDelta{}
	outMember := make([]MemberModeDelta, 0)
	if delta.Moderated == MODE_ADDED && !ch.Mode.Moderated {
		ch.Mode.Moderated = true
		outDelta.Moderated = MODE_ADDED
	} else if delta.Moderated == MODE_REMOVED && ch.Mode.Moderated {
		ch.Mode.Moderated = false
		outDelta.Moderated = MODE_REMOVED
	}
	if delta.NoExternalMessages == MODE_ADDED && !ch.Mode.NoExternalMessages {
		ch.Mode.NoExternalMessages = true
		outDelta.Moderated = MODE_ADDED
	} else if delta.NoExternalMessages == MODE_REMOVED && ch.Mode.NoExternalMessages {
		ch.Mode.NoExternalMessages = false
		outDelta.Moderated = MODE_REMOVED
	}
	if delta.Secret == MODE_ADDED && !ch.Mode.Secret {
		ch.Mode.Secret = true
		outDelta.Secret = MODE_ADDED
	} else if delta.Secret == MODE_REMOVED && ch.Mode.Secret {
		ch.Mode.Secret = false
		outDelta.Secret = MODE_REMOVED
	}
	if delta.TopicProtected == MODE_ADDED && !ch.Mode.TopicProtected {
		ch.Mode.TopicProtected = true
		outDelta.TopicProtected = MODE_ADDED
	} else if delta.TopicProtected == MODE_REMOVED && ch.Mode.TopicProtected {
		ch.Mode.TopicProtected = false
		outDelta.TopicProtected = MODE_REMOVED
	}
	for _, member := range memberDelta {
		membership, found := ch.Member[member.Client]
		if !found {
			// TODO: fix this.
			continue
		}
		outMemberDelta := MemberModeDelta{
			Client: member.Client,
		}
		if member.IsOwner == MODE_ADDED && !membership.IsOwner {
			membership.IsOwner = true
			outMemberDelta.IsOwner = MODE_ADDED
		} else if member.IsOwner == MODE_REMOVED && membership.IsOwner {
			membership.IsOwner = false
			outMemberDelta.IsOwner = MODE_REMOVED
		}
		if member.IsAdmin == MODE_ADDED && !membership.IsAdmin {
			membership.IsAdmin = true
			outMemberDelta.IsAdmin = MODE_ADDED
		} else if member.IsAdmin == MODE_REMOVED && membership.IsAdmin {
			membership.IsAdmin = false
			outMemberDelta.IsAdmin = MODE_REMOVED
		}
		if member.IsOp == MODE_ADDED && !membership.IsOp {
			membership.IsOp = true
			outMemberDelta.IsOp = MODE_ADDED
		} else if member.IsOp == MODE_REMOVED && membership.IsOp {
			membership.IsOp = false
			outMemberDelta.IsOp = MODE_REMOVED
		}
		if member.IsHalfop == MODE_ADDED && !membership.IsHalfop {
			membership.IsHalfop = true
			outMemberDelta.IsHalfop = MODE_ADDED
		} else if member.IsHalfop == MODE_REMOVED && membership.IsHalfop {
			membership.IsHalfop = false
			outMemberDelta.IsHalfop = MODE_REMOVED
		}
		if member.IsVoice == MODE_ADDED && !membership.IsVoice {
			membership.IsVoice = true
			outMemberDelta.IsVoice = MODE_ADDED
		} else if member.IsVoice == MODE_REMOVED && membership.IsVoice {
			membership.IsVoice = false
			outMemberDelta.IsVoice = MODE_REMOVED
		}

		if false ||
			outMemberDelta.IsOwner != MODE_UNCHANGED ||
			outMemberDelta.IsAdmin != MODE_UNCHANGED ||
			outMemberDelta.IsOp != MODE_UNCHANGED ||
			outMemberDelta.IsHalfop != MODE_UNCHANGED ||
			outMemberDelta.IsVoice != MODE_UNCHANGED {
			outMember = append(outMember, outMemberDelta)
		}
	}
	return outDelta, outMember
}

type ChannelModeDelta struct {
	TopicProtected, NoExternalMessages, Moderated, Secret ModeDelta
	Limit, Key                                            ModeDelta
	LimitValue                                            uint32
	KeyValue                                              string
}

func (cmd *ChannelModeDelta) IsEmpty() bool {
	return true &&
		cmd.Moderated == MODE_UNCHANGED &&
		cmd.NoExternalMessages == MODE_UNCHANGED &&
		cmd.Secret == MODE_UNCHANGED &&
		cmd.TopicProtected == MODE_UNCHANGED
}

func (cmd *ChannelModeDelta) String() string {
	return fmt.Sprintf("m=%s, n=%s, s=%s, t=%s", cmd.Moderated.String(), cmd.NoExternalMessages.String(), cmd.Secret.String(), cmd.TopicProtected.String())
}

type Membership struct {
	Ts                                        time.Time
	IsOwner, IsAdmin, IsOp, IsHalfop, IsVoice bool
}

func (m *Membership) Serialize(channel *Channel, client *Client) *SSMembership {
	return &SSMembership{
		Channel:  channel.Id(),
		Client:   client.Id(),
		Ts:       m.Ts,
		IsOwner:  m.IsOwner,
		IsAdmin:  m.IsAdmin,
		IsOp:     m.IsOp,
		IsHalfop: m.IsHalfop,
		IsVoice:  m.IsVoice,
	}
}

type MemberModeDelta struct {
	Client                                    *Client
	IsOwner, IsAdmin, IsOp, IsHalfop, IsVoice ModeDelta
}
