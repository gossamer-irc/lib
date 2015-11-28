package lib

import (
	"io"
	"log"
	"sync"
	"testing"
)

type testNetwork struct {
	root *Node
	all  map[string]*Node
	wg   *sync.WaitGroup
}

func newTestNetwork(rootServerName string, wg *sync.WaitGroup) (*testNetwork, *Node) {
	root := NewNode(Config{rootServerName, "Test Server", "TestNet", "test"}, nil, wg)
	tn := &testNetwork{
		root,
		make(map[string]*Node),
		wg,
	}
	tn.all[root.Me.Name] = root
	return tn, root
}

func (tn *testNetwork) NewServer(name string) *Node {
	return NewNode(Config{name, "Test Server", "TestNet", "test"}, nil, tn.wg)
}

func (tn *testNetwork) LinkNewServer(name string, to *Node) *Node {
	return tn.Link(tn.NewServer(name), to)
}

func (tn *testNetwork) Link(node *Node, to *Node) *Node {
	tn.all[node.Me.Name] = node

	// Two pipes for a duplex connection.
	abr, abw := io.Pipe()
	bar, baw := io.Pipe()

	sync := tn.setupVersionIncrementMonitor()

	// Make the actual connection.
	node.BeginLink(bar, abw, nil)
	to.BeginLink(abr, baw, nil)

	// Block until every server has processed this burst.
	<-sync

	return node
}

func (tn *testNetwork) Shutdown() {
	for _, node := range tn.all {
		node.Shutdown()
	}
}

func (tn *testNetwork) Sync() {
	tn.SyncFrom(tn.root)
}

func (tn *testNetwork) SyncFrom(node *Node) {
	<-node.Sync()
}

func (tn *testNetwork) SplitFromRoot(node *Node) *testNetwork {
	if node == tn.root {
		log.Fatalf("SplitFromRoot() on root.")
	}
	// Find the Node for the hub that's holding node.
	hub := tn.all[node.Network[tn.root.Me.Name].Route.Name]

	// Get a list of all other linked servers that are splitting off.
	toSplit := make([]string, 0)
	for _, server := range node.Network {
		if server.Route.Name == hub.Me.Name {
			toSplit = append(toSplit, server.Name)
		}
	}

	// Get the links to break on either side.
	linkRoot := hub.Network[node.Me.Name].Link
	linkSplit := node.Network[hub.Me.Name].Link

	sync := tn.setupVersionIncrementMonitor()

	linkRoot.writeChan.Close()
	linkSplit.writeChan.Close()

	<-sync

	// Create the split network, rooted at node.
	tnSplit := &testNetwork{
		root: node,
		all:  make(map[string]*Node),
		wg:   tn.wg,
	}

	// Move all split servers from tn to tnSplit.
	for _, name := range toSplit {
		tnSplit.all[name] = tn.all[name]
		delete(tn.all, name)
	}

	return tnSplit
}

// TODO(Alex): This doesn't work, as the version increment doesn't happen
// when deep linking a new server.
func (tn *testNetwork) setupVersionIncrementMonitor() chan struct{} {
	finalSignal := make(chan struct{})
	counter := make(chan struct{})

	count := len(tn.all)
	for _, node := range tn.all {
		node.versionMon = make(chan int)
		tn.wg.Add(1)
		go func(node *Node) {
			defer tn.wg.Done()
			<-node.versionMon
			node.versionMon = nil
			counter <- struct{}{}
		}(node)
	}

	tn.wg.Add(1)
	go func() {
		defer tn.wg.Done()
		recv := 0
		for _ = range counter {
			recv++
			if recv == count {
				finalSignal <- struct{}{}
				return
			}
		}
	}()

	return finalSignal
}

type tnsServer struct {
	name  string
	links []string
}

type testNetworkStructure struct {
	server map[string]*tnsServer
}

func newTestNetworkStructure(firstServer string) *testNetworkStructure {
	tns := &testNetworkStructure{
		server: make(map[string]*tnsServer),
	}
	tns.server[firstServer] = &tnsServer{
		name:  firstServer,
		links: make([]string, 0),
	}
	return tns
}

func (tns *testNetworkStructure) AddServer(name string, hub string) {
	s := &tnsServer{
		name:  name,
		links: make([]string, 0),
	}
	tns.server[name] = s
	s.links = append(s.links, hub)
	hs, found := tns.server[hub]
	if !found {
		log.Fatalf("No such hub: %s", hub)
	}
	hs.links = append(hs.links, name)
}

func (tns *testNetworkStructure) Validate(node *Node, t *testing.T) {
	root, found := tns.server[node.Me.Name]
	if !found {
		t.Errorf("[%s] root server not found in expected structure", node.Me.Name)
	}
	tns.validateHelper(node.Me.Name, node.Me.Name, node.Me, root, t)
}

func (tns *testNetworkStructure) validateHelper(nodeName string, rootName string, server *Server, tServer *tnsServer, t *testing.T) {
	for _, expected := range tServer.links {
		if expected == rootName {
			continue
		}
		link, found := server.Links[expected]
		if !found {
			t.Errorf("[%s] server %s not linked to %s as expected (with %s as root)", nodeName, expected, server.Name, rootName)
		} else {
			nextTServer, found := tns.server[expected]
			if !found {
				log.Fatalf("[%s] invalid test network structure", nodeName)
			}
			tns.validateHelper(nodeName, server.Name, link, nextTServer, t)
		}
	}
}
