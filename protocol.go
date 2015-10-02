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
	SS_MSG_TYPE_MEMBERSHIP
	SS_MSG_TYPE_PM
)

const (
	SS_KILL_REASON_COLLISION = iota
	SS_KILL_REASON_SENDQ     = iota
	SS_KILL_REASON_RECVQ     = iota
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
	constructorMap[SS_MSG_TYPE_PM] = func() SSMessage {
		return &SSPrivateMessage{}
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
	ReasonCode uint32
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
