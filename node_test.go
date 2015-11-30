package lib

import (
	"log"
	"sync"
	"testing"
	//"time"
)

func TestNodeLevelLink(t *testing.T) {
	wg := &sync.WaitGroup{}
	tn, hubA := newTestNetwork(t, "hub.a", wg)
	hubA.NewLink("hub.b")
	tn.Shutdown()
	wg.Wait()
}

func TestAttachSingle(t *testing.T) {
	wg := &sync.WaitGroup{}
	tn, hubA := newTestNetwork(t, "hub.a", wg)

	test := hubA.NewClient("TestUser")

	tn.ExpectAll(test.Exists())

	tn.Shutdown()
	wg.Wait()
}

func TestAttachDeepNetwork(t *testing.T) {
	wg := &sync.WaitGroup{}
	tn, hubA := newTestNetwork(t, "hub.a", wg)
	hubB := hubA.NewLink("hub.b")
	hubC := hubB.NewLink("hub.c")
	hubC.NewLink("hub.d")

	test := hubA.NewClient("TestUser")

	tn.ExpectAll(test.Exists())

	tn.Shutdown()
	wg.Wait()
}

func TestNickCollisionDuringLink(t *testing.T) {
	wg := &sync.WaitGroup{}
	tn, hubA := newTestNetwork(t, "hub.a", wg)

	hubB := tn.NewServer("hub.b")

	// Add 3 clients on A and the same 3 clients on B.
	// The first has an older timestamp on A, the last on B, and the middle
	// is tied.
	hubA.SetTS(1)
	hubB.SetTS(3)
	alphaA := hubA.NewClient("alpha")
	alphaB := hubB.NewClient("alpha")
	hubA.SetTS(2)
	hubB.SetTS(2)
	betaA := hubA.NewClient("beta")
	betaB := hubB.NewClient("beta")
	hubA.SetTS(3)
	hubB.SetTS(1)
	gammaA := hubA.NewClient("gamma")
	gammaB := hubB.NewClient("gamma")

	hubA.Link(hubB)

	tn.ExpectAll(alphaA.Exists())
	tn.ExpectAll(alphaB.Exists().Not())
	tn.ExpectAll(betaA.Exists().Not())
	tn.ExpectAll(betaB.Exists().Not())
	tn.ExpectAll(gammaA.Exists().Not())
	tn.ExpectAll(gammaB.Exists())

	tn.Shutdown()
	wg.Wait()
}

func TestNodeSplitBasic(t *testing.T) {
	wg := &sync.WaitGroup{}
	tnA, hubA := newTestNetwork(t, "hub.a", wg)
	hubB := hubA.NewLink("hub.b")
	tnB := tnA.SplitFromRoot(hubB)

	tnA.ExpectAll(hubB.IsLinked().Not())
	tnA.ExpectAll(hubB.Exists().Not())
	tnB.ExpectAll(hubA.IsLinked().Not())
	tnB.ExpectAll(hubA.Exists().Not())

	tnA.Shutdown()
	tnB.Shutdown()
	wg.Wait()
}

func TestNodeSplitMidChain(t *testing.T) {
	wg := &sync.WaitGroup{}
	tnA, hubA := newTestNetwork(t, "hub.a", wg)

	hubB := hubA.NewLink("hub.b")
	hubC := hubB.NewLink("hub.c")
	hubD := hubC.NewLink("hub.d")
	hubE := hubD.NewLink("hub.e")

	tnB := tnA.SplitFromRoot(hubC)

	hubB.Expect(hubC.IsLinked().Not())
	tnA.ExpectAll(hubC.Exists().Not())
	tnA.ExpectAll(hubC.IsLinked().Not())
	tnA.ExpectAll(hubD.Exists().Not())
	tnA.ExpectAll(hubE.Exists().Not())

	hubC.Expect(hubD.IsLinked())
	hubC.Expect(hubB.IsLinked().Not())
	tnB.ExpectAll(hubA.Exists().Not())
	tnB.ExpectAll(hubB.Exists().Not())
	tnB.ExpectAll(hubD.Exists())
	tnB.ExpectAll(hubE.Exists())

	tnA.Shutdown()
	tnB.Shutdown()
	wg.Wait()
}

func TestNodeSplitMoveServer(t *testing.T) {
	wg := &sync.WaitGroup{}
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

	tn, hubA := newTestNetwork(t, "hub.a", wg)
	hubB := hubA.NewLink("hub.b")
	hubB.NewLink("hub.c")
	hubD := hubA.NewLink("hub.d")
	hubA.NewLink("hub.e")

	// Validate initial configuration.
	tn.ExpectAll(initial)

	tn2 := tn.SplitFromRoot(hubB)

	// Validate intermediate configuration.
	tn.ExpectAll(afterSplit)

	// Reconnect.

	log.Printf("RELINK")
	hubB.Link(hubD)
	log.Printf("SYNC")
	tn.SyncFrom(hubD)

	tn.ExpectAll(afterLink)

	tn.Shutdown()
	tn2.Shutdown()
	wg.Wait()
}
