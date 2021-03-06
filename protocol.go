package lib

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

// Message types. Sent on the wire - do not reorder or delete types.
const (
	SS_MSG_TYPE_UNKNOWN uint32 = iota
	SS_MSG_TYPE_HELLO
	SS_MSG_TYPE_BURST_COMPLETE
	SS_MSG_TYPE_SYNC
	SS_MSG_TYPE_CLIENT
	SS_MSG_TYPE_SERVER
	SS_MSG_TYPE_KILL
	SS_MSG_TYPE_SPLIT
	SS_MSG_TYPE_CHANNEL
	SS_MSG_TYPE_CHANNEL_MODE
	SS_MSG_TYPE_MEMBERSHIP
	SS_MSG_TYPE_MEMBERSHIP_END
	SS_MSG_TYPE_PM
	SS_MSG_TYPE_CM
)

type SSKillReason uint8

const (
	SS_KILL_REASON_QUIT SSKillReason = iota
	SS_KILL_REASON_COLLISION
	SS_KILL_REASON_SENDQ
	SS_KILL_REASON_RECVQ
)

type ssMessageConstructor func() SSMessage

var constructorMap map[uint32]ssMessageConstructor

func init() {
	constructorMap = make(map[uint32]ssMessageConstructor)
	constructorMap[SS_MSG_TYPE_HELLO] = func() SSMessage {
		return &SSHello{}
	}
	constructorMap[SS_MSG_TYPE_BURST_COMPLETE] = func() SSMessage {
		return &SSBurstComplete{}
	}
	constructorMap[SS_MSG_TYPE_SYNC] = func() SSMessage {
		return &SSSync{}
	}
	constructorMap[SS_MSG_TYPE_CLIENT] = func() SSMessage {
		return &SSClient{}
	}
	constructorMap[SS_MSG_TYPE_SERVER] = func() SSMessage {
		return &SSServer{}
	}
	constructorMap[SS_MSG_TYPE_KILL] = func() SSMessage {
		return &SSKill{}
	}
	constructorMap[SS_MSG_TYPE_SPLIT] = func() SSMessage {
		return &SSSplit{}
	}
	constructorMap[SS_MSG_TYPE_CHANNEL] = func() SSMessage {
		return &SSChannel{}
	}
	constructorMap[SS_MSG_TYPE_MEMBERSHIP] = func() SSMessage {
		return &SSMembership{}
	}
	constructorMap[SS_MSG_TYPE_MEMBERSHIP_END] = func() SSMessage {
		return &SSMembershipEnd{}
	}
	constructorMap[SS_MSG_TYPE_PM] = func() SSMessage {
		return &SSPrivateMessage{}
	}
	constructorMap[SS_MSG_TYPE_CM] = func() SSMessage {
		return &SSChannelMessage{}
	}
	constructorMap[SS_MSG_TYPE_CHANNEL_MODE] = func() SSMessage {
		return &SSChannelMode{}
	}
}

var GobServerProtocolFactory ServerProtocolFactory = &gobServerProtocolFactory{}

// A Gossamer server-to-server message.
type SSMessage interface {
	String() string
	messageType() uint32
}

// Message header used to identify what message is being transmitted next.
type SSMessageHeader struct {
	Type uint32
}

/// Introduction message sent by one server to another at the beginning of link.
type SSHello struct {
	Protocol      uint32
	LocalTimeMs   uint64
	Name          string
	Description   string
	DefaultSubnet string
}

func (msg SSHello) String() string {
	return fmt.Sprintf("hello(%d, %d, %s, %s, %s)", msg.Protocol, msg.LocalTimeMs, msg.Name, msg.Description, msg.DefaultSubnet)
}

func (msg SSHello) messageType() uint32 {
	return SS_MSG_TYPE_HELLO
}

type SSBurstComplete struct {
	Server string
}

func (msg SSBurstComplete) String() string {
	return fmt.Sprintf("burstComplete(%s)", msg.Server)
}

func (msg SSBurstComplete) messageType() uint32 {
	return SS_MSG_TYPE_BURST_COMPLETE
}

type SSSync struct {
	Sequence  uint32
	Reply     bool
	Origin    string
	ReplyFrom string
}

func (msg SSSync) String() string {
	if msg.Reply {
		return fmt.Sprintf("syncReply(%s:%d, %s)", msg.Origin, msg.Sequence, msg.ReplyFrom)
	} else {
		return fmt.Sprintf("sync(%s:%d)", msg.Origin, msg.Sequence)
	}
}

func (msg SSSync) messageType() uint32 {
	return SS_MSG_TYPE_SYNC
}

type SSClient struct {
	Subnet                           string
	Server                           string
	Nick, Ident, Vident, Host, Vhost string
	Ip, Vip                          string
	Gecos                            string
	Ts                               time.Time
}

func (msg SSClient) String() string {
	return fmt.Sprintf("client(%s, %s, %s, ident(%s, %s), host(%s, %s), ip(%s, %s), %s, ts(%v))", msg.Subnet, msg.Server, msg.Nick, msg.Ident, msg.Vident, msg.Host, msg.Vhost, msg.Ip, msg.Vip, msg.Gecos, msg.Ts)
}

func (msg SSClient) messageType() uint32 {
	return SS_MSG_TYPE_CLIENT
}

type SSServer struct {
	Name string
	Desc string
	Via  string
}

func (msg SSServer) String() string {
	return fmt.Sprintf("server(%s, %s, via(%s))", msg.Name, msg.Desc, msg.Via)
}

func (msg SSServer) messageType() uint32 {
	return SS_MSG_TYPE_SERVER
}

type SSKill struct {
	// Id of the client being killed.
	Id SSClientId

	// Server doing the killing.
	Server string

	// Whether this kill message is instructional (server A telling server B to kill B's client)
	// or authoritative (server B broadcasting a kill of its own client)
	Authority bool

	// If this kill was initiated by another client, who it was.
	By SSClientId

	// Textual reason for the kill.
	Reason string

	// Numerical code indicating the reason for the kill (for statistics, etc).
	ReasonCode SSKillReason
}

func (msg SSKill) String() string {
	if msg.Authority {
		return fmt.Sprintf("kill(id(%v), server(%s), by(%v), reason(%s))", msg.Id, msg.Server, msg.By, msg.Server)
	} else {
		return fmt.Sprintf("kill?(id(%v), server(%s), by(%v), reason(%s))", msg.Id, msg.Server, msg.By, msg.Server)
	}
}

func (msg SSKill) messageType() uint32 {
	return SS_MSG_TYPE_KILL
}

type SSSplit struct {
	Server string
	Reason string
}

func (msg SSSplit) messageType() uint32 {
	return SS_MSG_TYPE_SPLIT
}

func (msg SSSplit) String() string {
	return fmt.Sprintf("split(%s, %s)", msg.Server, msg.Reason)
}

type SSChannel struct {
	Name    string
	Subnet  string
	Ts      time.Time
	Members []*SSMembership
}

func (msg SSChannel) messageType() uint32 {
	return SS_MSG_TYPE_CHANNEL
}

func (msg SSChannel) String() string {
	members := make([]string, len(msg.Members))
	for idx, member := range msg.Members {
		members[idx] = member.String()
	}
	return fmt.Sprintf("channel(%s, %s, %s, [%s])", msg.Name, msg.Subnet, msg.Ts, strings.Join(members, ", "))
}

type SSMembership struct {
	Client                                    SSClientId
	Channel                                   SSChannelId
	Ts                                        time.Time
	IsOwner, IsAdmin, IsOp, IsHalfop, IsVoice bool
}

func (msg SSMembership) messageType() uint32 {
	return SS_MSG_TYPE_MEMBERSHIP
}

func (msg SSMembership) String() string {
	modes := make([]string, 0, 5)
	if msg.IsOwner {
		modes = append(modes, "owner")
	}
	if msg.IsAdmin {
		modes = append(modes, "admin")
	}
	if msg.IsOp {
		modes = append(modes, "op")
	}
	if msg.IsHalfop {
		modes = append(modes, "halfop")
	}
	if msg.IsVoice {
		modes = append(modes, "voice")
	}
	return fmt.Sprintf("membership(%s, %s, [%s])", msg.Channel, msg.Client, strings.Join(modes, ", "))
}

type SSPrivateMessage struct {
	From    SSClientId
	To      SSClientId
	Message string
}

func (msg SSPrivateMessage) messageType() uint32 {
	return SS_MSG_TYPE_PM
}

func (msg SSPrivateMessage) String() string {
	return fmt.Sprintf("msg(%s -> %s, %s)", msg.From, msg.To, msg.Message)
}

type SSChannelMessage struct {
	From    SSClientId
	To      SSChannelId
	Message string
}

func (msg SSChannelMessage) messageType() uint32 {
	return SS_MSG_TYPE_CHANNEL
}

func (msg SSChannelMessage) String() string {
	return fmt.Sprintf("msg(%s -> %s, %s", msg.From, msg.To, msg.Message)
}

type SSMembershipEnd struct {
	Channel SSChannelId
	Client  SSClientId
	Reason  string
}

func (msg SSMembershipEnd) messageType() uint32 {
	return SS_MSG_TYPE_MEMBERSHIP_END
}

func (msg SSMembershipEnd) String() string {
	return fmt.Sprintf("part(%s, %s, %s)", msg.Channel, msg.Client, msg.Reason)
}

type SSModeDelta uint8

const (
	SS_MODE_UNCHANGED SSModeDelta = iota
	SS_MODE_ADDED
	SS_MODE_REMOVED
)

func SSModeDeltaFromModeDelta(value ModeDelta) SSModeDelta {
	switch value {
	case MODE_UNCHANGED:
		return SS_MODE_UNCHANGED
	case MODE_ADDED:
		return SS_MODE_ADDED
	case MODE_REMOVED:
		return SS_MODE_REMOVED
	default:
		panic("Unknown ModeDelta")
	}
}

func (value SSModeDelta) ToModeDelta() ModeDelta {
	switch value {
	case SS_MODE_UNCHANGED:
		return MODE_UNCHANGED
	case SS_MODE_ADDED:
		return MODE_ADDED
	case SS_MODE_REMOVED:
		return MODE_REMOVED
	default:
		panic("Unknown SSModeDelta")
	}
}

type SSMemberModeDelta struct {
	Client                                    SSClientId
	IsOwner, IsAdmin, IsOp, IsHalfop, IsVoice SSModeDelta
}

func SSMemberModeDeltaFromMemberModeDelta(value MemberModeDelta) SSMemberModeDelta {
	return SSMemberModeDelta{
		Client:   value.Client.Id(),
		IsOwner:  SSModeDeltaFromModeDelta(value.IsOwner),
		IsAdmin:  SSModeDeltaFromModeDelta(value.IsAdmin),
		IsOp:     SSModeDeltaFromModeDelta(value.IsOp),
		IsHalfop: SSModeDeltaFromModeDelta(value.IsHalfop),
		IsVoice:  SSModeDeltaFromModeDelta(value.IsVoice),
	}
}

func (delta SSMemberModeDelta) ToMemberModeDelta(n *Node) (MemberModeDelta, bool) {
	client, found := n.lookupClientById(delta.Client)
	if !found {
		return MemberModeDelta{}, false
	}
	return MemberModeDelta{
		Client:   client,
		IsOwner:  delta.IsOwner.ToModeDelta(),
		IsAdmin:  delta.IsAdmin.ToModeDelta(),
		IsOp:     delta.IsOp.ToModeDelta(),
		IsHalfop: delta.IsHalfop.ToModeDelta(),
		IsVoice:  delta.IsVoice.ToModeDelta(),
	}, true
}

type SSChannelModeDelta struct {
	TopicProtected, NoExternalMessages, Moderated, Secret SSModeDelta
	Limit, Key                                            SSModeDelta
	LimitValue                                            uint32
	KeyValue                                              string
}

func ChannelModeDeltaToSSChannelModeDelta(value ChannelModeDelta) SSChannelModeDelta {
	return SSChannelModeDelta{
		Key:                SSModeDeltaFromModeDelta(value.Key),
		Limit:              SSModeDeltaFromModeDelta(value.Limit),
		Moderated:          SSModeDeltaFromModeDelta(value.Moderated),
		NoExternalMessages: SSModeDeltaFromModeDelta(value.NoExternalMessages),
		Secret:             SSModeDeltaFromModeDelta(value.Secret),
		TopicProtected:     SSModeDeltaFromModeDelta(value.TopicProtected),
		KeyValue:           value.KeyValue,
		LimitValue:         value.LimitValue,
	}
}

func (delta SSChannelModeDelta) ToChannelModeDelta() ChannelModeDelta {
	return ChannelModeDelta{
		Key:                delta.Key.ToModeDelta(),
		Limit:              delta.Limit.ToModeDelta(),
		Moderated:          delta.Moderated.ToModeDelta(),
		NoExternalMessages: delta.NoExternalMessages.ToModeDelta(),
		Secret:             delta.Secret.ToModeDelta(),
		TopicProtected:     delta.TopicProtected.ToModeDelta(),
		KeyValue:           delta.KeyValue,
		LimitValue:         delta.LimitValue,
	}
}

type SSChannelMode struct {
	From       SSClientId
	Channel    SSChannelId
	Mode       SSChannelModeDelta
	MemberMode []SSMemberModeDelta
}

func (msg SSChannelMode) messageType() uint32 {
	return SS_MSG_TYPE_CHANNEL_MODE
}

func (msg SSChannelMode) String() string {
	return fmt.Sprintf("mode(%s, %s)", msg.Channel, msg.From)
}

// TODO Why does SSClientId have Server?
type SSClientId struct {
	Server string
	Subnet string
	Nick   string
}

func (id SSClientId) String() string {
	return fmt.Sprintf("%s:%s@%s", id.Subnet, id.Nick, id.Server)
}

type SSChannelId struct {
	Subnet string
	Name   string
}

func (id SSChannelId) String() string {
	return fmt.Sprintf("#%s:%s", id.Subnet, id.Name)
}

type ServerProtocolFactory interface {
	Reader(reader io.Reader) ServerProtocolReader
	Writer(writer io.Writer) ServerProtocolWriter
}

// A Gossamer server-to-server message reader.
type ServerProtocolReader interface {
	ReadMessage() (msg SSMessage, err error)
}

type gobServerProtocolReader struct {
	decoder *gob.Decoder
}

// Construct a ServerProtcolReader from the underlying io.Reader which interprets
// incoming data using the gob encoding.
func NewGobServerProtocolReader(reader io.Reader) ServerProtocolReader {
	dec := gob.NewDecoder(reader)
	return &gobServerProtocolReader{dec}
}

func (spr *gobServerProtocolReader) ReadMessage() (msg SSMessage, err error) {
	var header SSMessageHeader
	err = spr.decoder.Decode(&header)
	if err != nil {
		return
	}
	ctor, ok := constructorMap[header.Type]
	if !ok {
		log.Fatalf("Unknown message type: %d", header.Type)
	}

	msg = ctor()
	err = spr.decoder.Decode(msg)
	if err != nil {
		msg = nil
	}
	return
}

// A Gossamer server-to-server message writer.
type ServerProtocolWriter interface {
	WriteMessage(msg SSMessage) error
}

type gobServerProtocolWriter struct {
	encoder *gob.Encoder
}

// Construct a ServerProtocolWriter from the underlying io.Writer which serializes
// outgoing messages using the gob encoding.
func NewGobServerProtocolWriter(writer io.Writer) ServerProtocolWriter {
	enc := gob.NewEncoder(writer)
	return &gobServerProtocolWriter{enc}
}

func (spw *gobServerProtocolWriter) WriteMessage(msg SSMessage) (err error) {
	var header SSMessageHeader
	header.Type = msg.messageType()
	err = spw.encoder.Encode(header)
	if err != nil {
		return
	}
	err = spw.encoder.Encode(msg)
	return
}

type gobServerProtocolFactory struct{}

func (gspf gobServerProtocolFactory) Reader(reader io.Reader) ServerProtocolReader {
	return NewGobServerProtocolReader(reader)
}

func (gspf gobServerProtocolFactory) Writer(writer io.Writer) ServerProtocolWriter {
	return NewGobServerProtocolWriter(writer)
}
