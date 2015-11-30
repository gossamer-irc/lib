package lib

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

type testNetwork struct {
	t    *testing.T
	root *testServer
	all  map[string]*testServer
	wg   *sync.WaitGroup
}

type testServer struct {
	name string
	net  *testNetwork
	node *Node
	ts   int64
}

type testClient struct {
	host   *testServer
	client *Client
}

type testChannel struct {
	name string
}

func newTestNetwork(t *testing.T, rootServerName string, wg *sync.WaitGroup) (*testNetwork, *testServer) {
	tn := &testNetwork{
		t: t,
		root: &testServer{
			name: rootServerName,
		},
		all: make(map[string]*testServer),
		wg:  wg,
	}
	tn.root.net = tn
	tn.root.node = NewNode(Config{rootServerName, "Test Server", "TestNet", "test"}, nil, wg)
	tn.all[tn.root.node.Me.Name] = tn.root
	return tn, tn.root
}

func (tn *testNetwork) NewServer(name string) *testServer {
	server := &testServer{
		name: name,
		net:  tn,
	}
	server.node = NewNode(Config{name, "Test Server", "TestNet", "test"}, nil, tn.wg)
	return server
}

func (tn *testNetwork) NewChannel(name string) *testChannel {
	return &testChannel{name}
}

func (ts *testServer) NewLink(name string) *testServer {
	return ts.Link(ts.net.NewServer(name))
}

func (from *testServer) Link(to *testServer) *testServer {
	from.net.all[to.name] = to
	to.net = from.net

	// Two pipes for a duplex connection.
	abr, abw := io.Pipe()
	bar, baw := io.Pipe()

	sync := from.net.setupVersionIncrementMonitor()

	// Make the actual connection.
	from.node.Do(func() {
		from.node.BeginLink(bar, abw, nil, fmt.Sprintf("new(%s -> %s)", from.name, to.name))
	})
	to.node.Do(func() {
		to.node.BeginLink(abr, baw, nil, fmt.Sprintf("new(%s -> %s)", to.name, from.name))
	})
	log.Printf("Done")
	// Block until every server has processed this burst.
	<-sync
	log.Printf("Sync done")

	return to
}

func (ts *testServer) NewClient(nick string) *testClient {
	tc := &testClient{
		host: ts,
		client: &Client{
			Subnet: ts.node.DefaultSubnet,
			Nick:   nick,
			Ident:  nick,
			Host:   fmt.Sprintf("host.%s", nick),
			Gecos:  nick,
			Ts:     time.Unix(ts.ts, 0),
		},
	}
	ts.node.AttachClient(tc.client)
	ts.net.Sync()
	return tc
}

func (ts *testServer) SetTS(t int64) {
	ts.ts = t
}

func (ts *testServer) Exists() *serverExistsMatcher {
	return &serverExistsMatcher{ts}
}

func (ts *testServer) IsLinked() *serverLinkMatcher {
	return &serverLinkMatcher{ts}
}

func (ts *testServer) Expect(matcher testMatcher) {
	if !matcher.Apply(ts) {
		ts.net.t.Errorf("%s: failed to match %s", ts.name, matcher.String())
	}
}

func (tc *testClient) Join(tch *testChannel) {
	tc.host.node.Do(func() {
		_, err := tc.host.node.JoinOrCreateChannel(tc.client, tc.host.node.DefaultSubnet, tch.name)
		if err != nil {
			tc.host.net.t.Fatalf("Failure to join channel: %v", err)
		}
	})
	tc.host.net.Sync()
}

func (tc *testClient) findOn(node *Node) *Client {
	ch := make(chan *Client)
	node.Do(func() {
		client, found := node.DefaultSubnet.Client[tc.client.Lnick]
		if !found {
			ch <- nil
		} else {
			ch <- client
		}
	})

	client := <-ch
	if client == nil {
		tc.host.net.t.Fatalf("Failed to find client %s on server %s", tc.client.Nick, node.Me.Name)
	}
	return client
}

func (tc *testClient) SetChannelMode(tch *testChannel, mode string, arg ...interface{}) {
	channel, found := tc.host.node.DefaultSubnet.Channel[tch.name]
	if !found {
		tc.host.net.t.Fatalf("Channel %s not found", tch.name)
	}

	operation := MODE_UNCHANGED
	delta := ChannelModeDelta{}
	memberDelta := make([]MemberModeDelta, 0)
	argIdx := 0
	for _, r := range []rune(mode) {
		switch r {
		case '+':
			operation = MODE_ADDED
		case '-':
			operation = MODE_REMOVED
		case 'q':
		case 'a':
		case 'o':
		case 'h':
		case 'v':
			if len(arg) <= argIdx {
				tc.host.net.t.Fatalf("Missing argument for mode '%v'", r)
			}
			testTarget, ok := arg[argIdx].(*testClient)
			if !ok {
				tc.host.net.t.Fatalf("Wrong argument type for mode '%v': %v", r, arg)
			}
			argIdx++
			target := testTarget.findOn(tc.host.net.root.node)
			mdEntry := MemberModeDelta{
				Client: target,
			}
			switch r {
			case 'q':
				mdEntry.IsOwner = operation
			case 'a':
				mdEntry.IsAdmin = operation
			case 'o':
				mdEntry.IsOp = operation
			case 'h':
				mdEntry.IsHalfop = operation
			case 'v':
				mdEntry.IsVoice = operation
			default:
				tc.host.net.t.FailNow()
			}

			memberDelta = append(memberDelta, mdEntry)
		}
	}

	tc.host.node.Do(func() {
		tc.host.node.ChangeChannelMode(tc.client, channel, delta, memberDelta)
	})
	tc.host.net.Sync()
}

func (tc *testClient) Exists() *clientExistsMatcher {
	return &clientExistsMatcher{tc}
}

func (tch *testChannel) Member(tc *testClient) *membershipSelector {
	return &membershipSelector{
		channel: tch,
		target:  tc,
	}
}

func (tn *testNetwork) Shutdown() {
	for _, ts := range tn.all {
		ts.node.Shutdown()
	}
}

func (tn *testNetwork) Sync() {
	tn.SyncFrom(tn.root)
}

func (tn *testNetwork) SyncFrom(ts *testServer) {
	<-ts.node.Sync()
}

func (tn *testNetwork) SplitFromRoot(ts *testServer) *testNetwork {
	// Can't split the root server.
	if ts == tn.root {
		log.Fatalf("SplitFromRoot() on root.")
	}

	// Find the testServer to which the splitting server is connected
	hub := tn.all[tn.root.node.Network[ts.name].Hub.Name]

	// Figure out which servers will be split.
	toSplit := testSplitHelper(hub.name, ts)

	// Get the near and far side of the Link.
	near := hub.node.Network[ts.name].Link
	far := ts.node.Network[hub.name].Link

	// Begin monitoring for the split.
	sync := tn.setupVersionIncrementMonitor()

	// Netsplit!
	near.writeChan.Close()
	far.writeChan.Close()

	// Wait for the network to synchronize.
	<-sync

	// Create the split network, rooted at the split server.
	tnSplit := &testNetwork{
		wg:   tn.wg,
		all:  make(map[string]*testServer),
		t:    tn.t,
		root: ts,
	}

	// Move all split servers to the new split network.
	for _, server := range toSplit {
		tnSplit.all[server.name] = server
		server.net = tnSplit
		delete(tn.all, server.name)
	}

	tn.Sync()
	tnSplit.Sync()

	return tnSplit
}

func testSplitHelper(near string, far *testServer) []*testServer {
	res := make([]*testServer, 1)
	res[0] = far

	for _, server := range far.node.Local {
		if server.Name == near {
			continue
		}
		ts, found := far.net.all[server.Name]
		if !found {
			log.Fatalf("Couldn't find record of server %s on the network", server.Name)
		}
		res = append(res, testSplitHelper(far.name, ts)...)
	}

	return res
}

// TODO(Alex): This doesn't work, as the version increment doesn't happen
// when deep linking a new server.
func (tn *testNetwork) setupVersionIncrementMonitor() chan struct{} {
	finalSignal := make(chan struct{})
	counter := make(chan struct{})

	count := len(tn.all)
	for _, server := range tn.all {
		node := server.node
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

func (tn *testNetwork) ExpectAll(matcher testMatcher) {
	for _, server := range tn.all {
		server.Expect(matcher)
	}
}

type membershipSelector struct {
	channel *testChannel
	target  *testClient
}

func (ms *membershipSelector) Select(ts *testServer) *Membership {
	client := ms.target.findOn(ts.node)
	channel, found := ts.node.DefaultSubnet.Channel[ms.channel.name]
	if !found {
		// Not finding the channel is considered not finding the member,
		// since during splits some servers may not have the channel
		// anymore.
		return nil
	}
	membership, found := channel.Member[client]
	if !found {
		return nil
	}
	return membership
}

func (ms *membershipSelector) String() string {
	return fmt.Sprintf("member(#%s, %s)", ms.channel.name, ms.target.client.Nick)
}

func (ms *membershipSelector) IsOwner() *modeMatcher {
	return (&modeMatcher{
		selector: ms,
	}).IsOwner()
}

func (ms *membershipSelector) IsAdmin() *modeMatcher {
	return (&modeMatcher{
		selector: ms,
	}).IsAdmin()
}

func (ms *membershipSelector) IsOp() *modeMatcher {
	return (&modeMatcher{
		selector: ms,
	}).IsOp()
}

func (ms *membershipSelector) IsHalfop() *modeMatcher {
	return (&modeMatcher{
		selector: ms,
	}).IsHalfop()
}

func (ms *membershipSelector) IsVoice() *modeMatcher {
	return (&modeMatcher{
		selector: ms,
	}).IsVoice()
}

func (ms *membershipSelector) Exists() *memberExistsMatcher {
	return &memberExistsMatcher{ms}
}

type testMatcher interface {
	Apply(ts *testServer) bool
	Not() testMatcher
	String() string
}

type notMatcher struct {
	matcher testMatcher
}

func (nm *notMatcher) Apply(ts *testServer) bool {
	return !nm.matcher.Apply(ts)
}

func (nm *notMatcher) Not() testMatcher {
	return &notMatcher{nm}
}

func (nm *notMatcher) String() string {
	return fmt.Sprintf("not(%s)", nm.matcher.String())
}

type memberExistsMatcher struct {
	selector *membershipSelector
}

func (mem *memberExistsMatcher) Apply(ts *testServer) bool {
	return mem.selector.Select(ts) != nil
}

func (mem *memberExistsMatcher) Not() testMatcher {
	return &notMatcher{mem}
}

func (mem *memberExistsMatcher) String() string {
	return fmt.Sprintf("exists(%s)", mem.selector.String())
}

type modeMatcher struct {
	selector                        *membershipSelector
	owner, admin, op, halfop, voice bool
}

func (mm *modeMatcher) Apply(ts *testServer) bool {
	mship := mm.selector.Select(ts)
	if mship == nil {
		return false
	}
	return true &&
		(!mm.owner || mship.IsOwner) &&
		(!mm.admin || mship.IsAdmin) &&
		(!mm.op || mship.IsOp) &&
		(!mm.halfop || mship.IsHalfop) &&
		(!mm.voice || mship.IsVoice)
}

func (mm *modeMatcher) Not() testMatcher {
	return &notMatcher{mm}
}

func (mm *modeMatcher) String() string {
	modes := make([]string, 0)
	if mm.owner {
		modes = append(modes, "owner")
	}
	if mm.admin {
		modes = append(modes, "admin")
	}
	if mm.op {
		modes = append(modes, "op")
	}
	if mm.halfop {
		modes = append(modes, "halfop")
	}
	if mm.voice {
		modes = append(modes, "voice")
	}
	return fmt.Sprintf("mode(%s, [%s])", mm.selector.String(), strings.Join(modes, ", "))
}

func (mm *modeMatcher) IsOwner() *modeMatcher {
	mm.owner = true
	return mm
}

func (mm *modeMatcher) IsAdmin() *modeMatcher {
	mm.admin = true
	return mm
}

func (mm *modeMatcher) IsOp() *modeMatcher {
	mm.op = true
	return mm
}

func (mm *modeMatcher) IsHalfop() *modeMatcher {
	mm.halfop = true
	return mm
}

func (mm *modeMatcher) IsVoice() *modeMatcher {
	mm.voice = true
	return mm
}

type clientExistsMatcher struct {
	client *testClient
}

func (cem *clientExistsMatcher) Apply(ts *testServer) bool {
	client, found := ts.node.DefaultSubnet.Client[cem.client.client.Lnick]
	if !found {
		return false
	}
	if cem.client.host == ts {
		// This is the host - strongest test possible
		return client == cem.client.client
	}
	return client.Server.Name == cem.client.host.name
}

func (cem *clientExistsMatcher) Not() testMatcher {
	return &notMatcher{cem}
}

func (cem *clientExistsMatcher) String() string {
	return fmt.Sprintf("exists(%s)", cem.client.client.Nick)
}

type serverLinkMatcher struct {
	target *testServer
}

func (slm *serverLinkMatcher) Apply(ts *testServer) bool {
	_, found := ts.node.Me.Links[slm.target.name]
	return found
}

func (slm *serverLinkMatcher) Not() testMatcher {
	return &notMatcher{slm}
}

func (slm *serverLinkMatcher) String() string {
	return fmt.Sprintf("linked(%s)", slm.target.name)
}

type serverExistsMatcher struct {
	target *testServer
}

func (sem *serverExistsMatcher) Apply(ts *testServer) bool {
	_, found := ts.node.Network[sem.target.name]
	return found
}

func (sem *serverExistsMatcher) Not() testMatcher {
	return &notMatcher{sem}
}

func (sem *serverExistsMatcher) String() string {
	return fmt.Sprintf("exists(%s)", sem.target.name)
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

func (tns *testNetworkStructure) Apply(ts *testServer) bool {
	es, found := tns.server[ts.name]
	if !found {
		// Unexpected server
		return false
	}
	for _, link := range es.links {
		_, found = ts.node.Me.Links[link]
		if !found {
			return false
		}
	}
	return true
}

func (tns *testNetworkStructure) Not() testMatcher {
	return &notMatcher{tns}
}

func (tns *testNetworkStructure) String() string {
	return fmt.Sprintf("testNetworkStructure()")
}
