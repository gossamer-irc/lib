package lib

import (
	"testing"
	"time"
)

func TestNodeLevelLink(t *testing.T) {
	tn, hubA := newTestNetwork("hub.a")
	defer tn.Shutdown()
	tn.LinkNewServer("hub.b", hubA)
}

func TestAttachSingle(t *testing.T) {
	tn, hubA := newTestNetwork("hub.a")
	defer tn.Shutdown()

	c := &Client{
		Nick:   "TestUser",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "Testing Gecos",
		Subnet: hubA.DefaultSubnet,
	}
	hubA.AttachClient(c)

	user, found := hubA.DefaultSubnet.Client["testuser"]
	if !found {
		t.Fatalf("Unable to find user in default subnet.")
	}
	if user != c {
		t.Errorf("Wrong user: %s", user.DebugString())
	}
}

func TestAttachDeepNetwork(t *testing.T) {
	tn, hubA := newTestNetwork("hub.a")
	defer tn.Shutdown()

	hubB := tn.LinkNewServer("hub.b", hubA)
	hubC := tn.LinkNewServer("hub.c", hubB)
	hubD := tn.LinkNewServer("hub.d", hubC)

	hubA.AttachClient(&Client{
		Nick:   "TestUser",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "Testing Gecos",
		Subnet: hubA.DefaultSubnet,
	})

	tn.Sync()

	user, found := hubD.DefaultSubnet.Client["testuser"]
	if !found {
		t.Fatalf("Unable to find user in default subnet of hub.d.")
	}
	if user.Nick != "TestUser" {
		t.Errorf("Wrong user: %s", user.DebugString())
	}
}

func TestNickCollisionDuringLink(t *testing.T) {
	tn, hubA := newTestNetwork("hub.a")
	defer tn.Shutdown()

	hubB := tn.NewServer("hub.b")

	// Add 3 clients on A and the same 3 clients on B.
	// The first has an older timestamp on A, the last on B, and the middle
	// is tied.
	hubA.AttachClient(&Client{
		Nick:   "TestUser1",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "On hub.a",
		Subnet: hubA.DefaultSubnet,
		Ts:     time.Unix(1, 0),
	})
	hubA.AttachClient(&Client{
		Nick:   "TestUser2",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "On hub.a",
		Subnet: hubA.DefaultSubnet,
		Ts:     time.Unix(2, 0),
	})
	hubA.AttachClient(&Client{
		Nick:   "TestUser3",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "On hub.a",
		Subnet: hubA.DefaultSubnet,
		Ts:     time.Unix(3, 0),
	})

	hubB.AttachClient(&Client{
		Nick:   "TestUser1",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "On hub.b",
		Subnet: hubB.DefaultSubnet,
		Ts:     time.Unix(3, 0),
	})
	hubB.AttachClient(&Client{
		Nick:   "TestUser2",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "On hub.b",
		Subnet: hubB.DefaultSubnet,
		Ts:     time.Unix(2, 0),
	})
	hubB.AttachClient(&Client{
		Nick:   "TestUser3",
		Ident:  "test",
		Host:   "testing.host",
		Gecos:  "On hub.b",
		Subnet: hubB.DefaultSubnet,
		Ts:     time.Unix(1, 0),
	})

	tn.Link(hubB, hubA)
	tn.Sync()

	client, found := hubA.DefaultSubnet.Client["testuser1"]
	if !found || client.Server.Name != "hub.a" {
		t.Errorf("testuser1 on hub.a not found or on wrong server.")
	}
	client, found = hubB.DefaultSubnet.Client["testuser1"]
	if !found || client.Server.Name != "hub.a" {
		t.Errorf("testuser1 on hub.b not found or on wrong server.")
	}

	_, found = hubA.DefaultSubnet.Client["testuser2"]
	if found {
		t.Errorf("testuser2 on hub.a found when it should have been killed.")
	}
	_, found = hubB.DefaultSubnet.Client["testuser2"]
	if found {
		t.Errorf("testuser2 on hub.b found when it should have been killed.")
	}

	client, found = hubA.DefaultSubnet.Client["testuser3"]
	if !found || client.Server.Name != "hub.b" {
		t.Errorf("testuser3 on hub.a not found or on wrong server.")
	}
	client, found = hubB.DefaultSubnet.Client["testuser3"]
	if !found || client.Server.Name != "hub.b" {
		t.Errorf("testuser3 on hub.b not found or on wrong server.")
	}
}

func TestNodeSplitBasic(t *testing.T) {
	tnA, hubA := newTestNetwork("hub.a")
	defer tnA.Shutdown()

	hubB := tnA.NewServer("hub.b")
	tnA.Link(hubB, hubA)
	tnA.Sync()

	tnB := tnA.SplitFromRoot(hubB)
	defer tnB.Shutdown()

	tnA.Sync()

	_, found := hubA.Network["hub.b"]
	if found {
		t.Errorf("hub.b found still referenced on hub.a, should not be.")
	}
	_, found = hubB.Network["hub.a"]
	if found {
		t.Errorf("hub.a found still referenced on hub.b, should not be.")
	}
	_, found = hubA.Me.Links["hub.b"]
	if found {
		t.Errorf("hub.b found still linked to hub.a, should not be.")
	}
	_, found = hubB.Me.Links["hub.a"]
	if found {
		t.Errorf("hub.a found still linked on hub.b, should not be.")
	}
}

func TestNodeSplitMidChain(t *testing.T) {
	tnA, hubA := newTestNetwork("hub.a")
	defer tnA.Shutdown()

	hubB := tnA.NewServer("hub.b")
	hubC := tnA.NewServer("hub.c")
	hubD := tnA.NewServer("hub.d")
	hubE := tnA.NewServer("hub.e")
	tnA.Link(hubB, hubA)
	tnA.Link(hubC, hubB)
	tnA.Link(hubD, hubC)
	tnA.Link(hubE, hubD)

	tnA.Sync()

	tnB := tnA.SplitFromRoot(hubC)
	defer tnB.Shutdown()

	_, found := hubA.Network["hub.c"]
	if found {
		t.Errorf("hub.c found still referenced on hub.a, should not be.")
	}
	_, found = hubA.Network["hub.d"]
	if found {
		t.Errorf("hub.d found still referenced on hub.a, should not be.")
	}
	_, found = hubA.Network["hub.e"]
	if found {
		t.Errorf("hub.e found still referenced on hub.a, should not be.")
	}
}

func TestNodeSplitMoveServer(t *testing.T) {
	// Structure we expect to see initially.
	// hub.a
	//   hub.b
	//     hub.c
	//   hub.d
	//   hub.e
	initial := newTestNetworkStructure("hub.a")
	initial.AddServer("hub.b", "hub.a")
	initial.AddServer("hub.c", "hub.b")
	initial.AddServer("hub.d", "hub.a")
	initial.AddServer("hub.e", "hub.a")

	// Structure we expect to see after splitting hub.b away.
	// hub.a
	//   hub.d
	//   hub.e
	afterSplit := newTestNetworkStructure("hub.a")
	afterSplit.AddServer("hub.d", "hub.a")
	afterSplit.AddServer("hub.e", "hub.a")

	// Structure we expect to see after linking hub.b to hub.d.
	// hub.a
	//   hub.d
	//     hub.b
	//       hub.c
	//   hub.e
	afterLink := newTestNetworkStructure("hub.a")
	afterLink.AddServer("hub.d", "hub.a")
	afterLink.AddServer("hub.b", "hub.d")
	afterLink.AddServer("hub.c", "hub.b")
	afterLink.AddServer("hub.e", "hub.a")

	tn, hubA := newTestNetwork("hub.a")
	defer tn.Shutdown()
	hubB := tn.NewServer("hub.b")
	hubC := tn.NewServer("hub.c")
	hubD := tn.NewServer("hub.d")
	hubE := tn.NewServer("hub.e")
	tn.Link(hubB, hubA)
	tn.Link(hubC, hubB)
	tn.Link(hubD, hubA)
	tn.Link(hubE, hubA)

	tn.Sync()

	// Validate initial configuration.
	initial.Validate(hubA, t)
	initial.Validate(hubB, t)
	initial.Validate(hubC, t)
	initial.Validate(hubD, t)
	initial.Validate(hubE, t)

	tn2 := tn.SplitFromRoot(hubB)
	tn.Sync()
	tn2.Sync()

	// Validate intermediate configuration.
	afterSplit.Validate(hubA, t)
	afterSplit.Validate(hubD, t)
	afterSplit.Validate(hubE, t)
	tn.Link(hubB, hubD)
	tn.SyncFrom(hubD)

	afterLink.Validate(hubA, t)
	afterLink.Validate(hubB, t)
	afterLink.Validate(hubC, t)
	afterLink.Validate(hubD, t)
	afterLink.Validate(hubE, t)

	tn.Sync()
}
