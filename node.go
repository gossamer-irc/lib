package lib

import (
	"io"
	"log"
	"strings"
	"sync"
	"time"
)

// Configuration spec for this node.
type Config struct {
	// Basic server name and info.
	ServerName, ServerDesc string

	// Network name.
	NetName string

	// Name of the default subnet (must match for networks to link)
	DefaultSubnetName string
}

// Represents a Gossamer distributed node's current state.
type Node struct {
	// Configuration
	config Config

	// Map of servers directly connected to this node.
	Local map[*Link]*Server

	// Map of all servers in the network.
	Network map[string]*Server

	NewLinks map[*Link]newLink

	Subnet map[string]*Subnet

	linkRecv chan LinkMessage
	exit     chan struct{}

	version    int
	versionMon chan int

	syncId      uint32
	syncsActive map[uint32]*syncRecord

	DefaultSubnet *Subnet
	Me            *Server
	Handler       EventHandler

	wg         *sync.WaitGroup
	linkReadWg *sync.WaitGroup
}

func NewNode(config Config, handler EventHandler, wg *sync.WaitGroup) *Node {
	validateConfig(config)

	node := &Node{
		config: config,

		linkRecv: make(chan LinkMessage),
		exit:     make(chan struct{}, 1),

		Local:         make(map[*Link]*Server),
		Network:       make(map[string]*Server),
		Subnet:        make(map[string]*Subnet),
		NewLinks:      make(map[*Link]newLink),
		version:       0,
		DefaultSubnet: NewSubnet(config.DefaultSubnetName),
		syncsActive:   make(map[uint32]*syncRecord),
		Me:            NewLocalServer(config.ServerName, config.ServerDesc, nil, nil),

		// ProxyEventHandler wrapper deals with nil handlers (which are allowed).
		Handler:    &ProxyEventHandler{handler},
		wg:         wg,
		linkReadWg: &sync.WaitGroup{},
	}
	node.Network[node.Me.Name] = node.Me
	node.Subnet[node.DefaultSubnet.Name] = node.DefaultSubnet
	wg.Add(1)
	node.linkReadWg.Add(1)
	go node.run()
	go node.linkReadClose()
	return node
}

func (n *Node) linkReadClose() {
	n.linkReadWg.Wait()
	close(n.linkRecv)
}

func (n *Node) run() {
	defer n.wg.Done()
	for {
		select {
		case <-n.exit:
			n.linkReadWg.Done()
			for _ = range n.linkRecv {
			}
			return
		case msg := <-n.linkRecv:
			if msg.link == nil {
				log.Fatalf("[%s] Nil link?", n.Me.Name)
			}
			if msg.link.Silence {
				log.Printf("[%s] Ignoring message from silenced link {%s}", n.Me.Name, msg.link.name)
				continue
			}
			// First, determine where this came from.
			nl, found := n.NewLinks[msg.link]
			if found {
				n.handleNewLinkMessage(msg, nl)
			} else {
				server, found := n.Local[msg.link]
				if !found {
					log.Fatalf("[%s] Expected to find server for link {%s}: [%v].", n.Me.Name, msg.link.name, msg.err)
				}

				if msg.err != nil {
					log.Printf("[%s] Error [%v] over link {%s} from server %s", n.Me.Name, msg.err, msg.link.name, server.Name)
					// Error on the link. Need to split.
					n.split(msg.link, msg.err)
					continue
				}

				n.handleLinkMessage(msg.msg, server)
			}
		}
	}
}

func (n *Node) bumpVersion() {
	n.version++
	if n.versionMon != nil {
		select {
		case n.versionMon <- n.version:
			// Do nothing.
			log.Printf("[%s] bumped version", n.Me.Name)
		default:
			log.Fatalf("[%s] went to increment version, but nothing listening", n.Me.Name)
		}
	}
}

func (n *Node) split(link *Link, err error) {
	// First, determine the server to which this link connects.
	server, found := n.Local[link]
	if !found {
		log.Fatalf("Got split() for link {%s} that doesn't exist for error: %v", link.name, err)
	}

	delete(n.Local, link)
	link.Close()
	link.Silence = true

	// Next, break the link, and resolve the consequences.
	n.processSplit(server, err.Error())

	// Finally, broadcast the split and increment the current version.
	n.SendAll(&SSSplit{server.Name, err.Error()})

	n.bumpVersion()
	log.Printf("[%s] split from %s: %v", n.Me.Name, server.Name, err)
}

func (n *Node) processSplit(server *Server, err string) {
	delete(n.Network, server.Name)
	delete(server.Hub.Links, server.Name)

	for _, subnet := range n.Subnet {
		removeList := make([]*Client, 0)
		for _, client := range subnet.Client {
			if client.Server == server {
				removeList = append(removeList, client)
			}
		}
		for _, client := range removeList {
			delete(client.Subnet.Client, client.Lnick)
		}
	}

	for _, linked := range server.Links {
		n.processSplit(linked, err)
	}
}

func (n *Node) processQuit(client *Client, reason string) {
	delete(client.Subnet.Client, client.Lnick)
}

func (n *Node) BurstTo(newServer *Server) {
	for _, server := range n.Local {
		n.burstServerHelper(newServer, server)
	}
	for _, subnet := range n.Subnet {
		for _, client := range subnet.Client {
			newServer.Send(client.Serialize())
		}
	}
	newServer.Send(SSBurstComplete{n.Me.Name})
}

func (n *Node) burstServerHelper(newServer *Server, hub *Server) {
	newServer.Send(hub.Serialize())
	for _, server := range hub.Links {
		n.burstServerHelper(newServer, server)
	}
}

func (n *Node) Shutdown() {
	for link, _ := range n.Local {
		link.Shutdown()
	}
	for link, _ := range n.NewLinks {
		link.Close()
	}
	n.exit <- struct{}{}
}

func (n *Node) AttachClient(client *Client) error {
	if client.Server != nil {
		log.Fatalf("Client is already attached [%s]", client.DebugString())
	}
	if client.Subnet == nil {
		log.Fatalf("No client subnet in AttachClient [%s]", client.DebugString())
	}
	client.Lnick = strings.ToLower(client.Nick)

	_, found := client.Subnet.Client[client.Lnick]
	if found {
		return NameInUseError{}
	}

	client.Server = n.Me
	client.Subnet.Client[client.Lnick] = client
	if client.Ts.IsZero() {
		client.Ts = time.Now().UTC()
	}

	log.Printf("[%s] attaching client: %s", n.Me.Name, client.DebugString())
	n.SendAll(client.Serialize())
	return nil
}

func (n *Node) AttachChannel(channel *Channel) error {
	if channel.Subnet == nil {
		log.Fatalf("No channel subnet in AttachChannel [%s]", channel.DebugString())
	}
	channel.Lname = strings.ToLower(channel.Name)

	_, found := channel.Subnet.Channel[channel.Lname]
	if found {
		return NameInUseError{}
	}

	channel.Subnet.Channel[channel.Lname] = channel
	if channel.Ts.IsZero() {
		channel.Ts = time.Now().UTC()
	}

	n.SendAll(channel.Serialize())
	return nil
}

func (n *Node) PrivateMessage(from, to *Client, message string) {
	if !from.IsLocal() {
		return
	}
	if to.IsLocal() {
		n.Handler.OnPrivateMessage(from, to, message)
	} else {
		to.Server.Route.Send(&SSPrivateMessage{from.Id(), to.Id(), message})
	}
}

func (n *Node) SendAll(msg SSMessage) {
	n.SendAllSkip(msg, nil)
}

func (n *Node) SendAllSkip(msg SSMessage, skip *Server) {
	for _, server := range n.Local {
		if server != skip {
			server.Send(msg)
		}
	}
}

func (n *Node) Sync() chan struct{} {
	n.syncId++

	sr := &syncRecord{
		synced:  make(chan struct{}, 1),
		servers: make(map[string]bool),
	}

	n.syncsActive[n.syncId] = sr

	for name, server := range n.Network {
		if server != n.Me {
			sr.servers[name] = false
		}
	}

	n.SendAll(&SSSync{
		Origin:   n.Me.Name,
		Sequence: n.syncId,
	})

	if len(sr.servers) == 0 {
		sr.synced <- struct{}{}
	}

	return sr.synced
}

func (n *Node) lookupClientById(id SSClientId) (client *Client, found bool) {
	sn, found := n.Subnet[id.Subnet]
	if !found {
		return
	}
	client, found = sn.Client[id.Nick]

	// Client ids contain the name of the server hosting the client. They must match
	// in order for the client to be considered found, otherwise the client mentioned
	// no longer exists.
	if client.Server.Name != id.Server {
		client = nil
		found = false
	}
	return
}

func (n *Node) NetworkName() string {
	return n.config.NetName
}

func validateConfig(config Config) {
	if config.ServerName == "" {
		log.Fatalf("No server name specified in configuration.")
	}
	if config.ServerDesc == "" {
		log.Fatalf("No server description specified in configuration.")
	}
	if config.NetName == "" {
		log.Fatalf("No network name specified in configuration.")
	}
	if config.DefaultSubnetName == "" {
		log.Fatalf("No default subnet name specified in configuration.")
	}
}

type newLink struct {
	link   *Link
	logger io.Writer
}

type syncRecord struct {
	synced  chan struct{}
	servers map[string]bool
}
