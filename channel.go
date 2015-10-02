package lib

import (
	"fmt"
	"time"
)

type ModeDelta uint8

const (
	MODE_UNCHANGED ModeDelta = iota
	MODE_ADDED
	MODE_REMOVED
)

type Channel struct {
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

func NewChannel(subnet *Subnet, name string) *Channel {
	return &Channel{
		Subnet:      subnet,
		Name:        name,
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
		Members: make([]*SSMembership, len(ch.Member)),
	}
	idx := 0
	for _, member := range ch.Member {
		msg.Members[idx] = member.Serialize(nil, nil)
		idx++
	}
	return msg
}

type ChannelModeDelta struct {
	TopicProtected, NoExternalMessages, Moderated, Secret ModeDelta
	Limit, Key                                            ModeDelta
	LimitValue                                            uint32
	KeyValue                                              string
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
	IsOwner, IsAdmin, IsOp, IsHalfop, IsVoice ModeDelta
}
