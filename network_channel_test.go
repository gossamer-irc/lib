package lib

import (
	"sync"
	"testing"
)

func TestNetworkChannel_Simple(t *testing.T) {
	wg := &sync.WaitGroup{}
	tn, hubA := newTestNetwork(t, "hub.a", wg)
	hubB := hubA.NewLink("hub.b")

	alpha := hubA.NewClient("alpha")
	beta := hubB.NewClient("beta")

	test := tn.NewChannel("test")
	alpha.Join(test)
	beta.Join(test)

	tn.ExpectAll(test.Member(alpha).IsOwner())
	tn.ExpectAll(test.Member(beta).IsOwner().Not())

	tn.Shutdown()
	wg.Wait()
}

func TestNetworkChannel_ModeChange(t *testing.T) {
	var wg sync.WaitGroup
	tn, hubA := newTestNetwork(t, "hub.a", &wg)
	hubB := hubA.NewLink("hub.b")

	// Two clients
	alpha := hubA.NewClient("alpha")
	beta := hubB.NewClient("beta")

	// Both join #test
	test := tn.NewChannel("test")
	alpha.Join(test)
	beta.Join(test)

	// Alpha voices beta
	alpha.SetChannelMode(test, "+v", beta)

	// Verify modes of both users
	tn.ExpectAll(test.Member(alpha).IsOwner())
	tn.ExpectAll(test.Member(beta).IsVoice())

	// Cleanup
	tn.Shutdown()
	wg.Wait()
}

func TestNetworkChannel_JoinPart(t *testing.T) {
	var wg sync.WaitGroup
	tn, hubA := newTestNetwork(t, "hub.a", &wg)
	hubB := hubA.NewLink("hub.b")

	alpha := hubA.NewClient("alpha")
	beta := hubB.NewClient("beta")

	test := tn.NewChannel("test")
	alpha.Join(test)
	beta.Join(test)
	beta.Part(test, "Leaving!")

	tn.ExpectAll(test.Member(alpha).Exists())
	tn.ExpectAll(test.Member(beta).Exists().Not())

	tn.Shutdown()
	wg.Wait()
}

func TestNetworkChannel_FarJoin(t *testing.T) {
	var wg sync.WaitGroup
	tn, hubA := newTestNetwork(t, "hub.a", &wg)
	hubC := hubA.NewLink("hub.b").NewLink("hub.c")

	alpha := hubA.NewClient("alpha")
	beta := hubC.NewClient("beta")

	test := tn.NewChannel("test")
	alpha.Join(test)
	beta.Join(test)

	tn.ExpectAll(test.Member(alpha).Exists())
	tn.ExpectAll(test.Member(beta).Exists())

	tn.Shutdown()
	wg.Wait()
}

func TestNetworkChannel_FarOwnerDeowner(t *testing.T) {
	var wg sync.WaitGroup
	tn, hubA := newTestNetwork(t, "hub.a", &wg)
	hubC := hubA.NewLink("hub.b").NewLink("hub.c")

	alpha := hubA.NewClient("alpha")
	beta := hubC.NewClient("beta")

	test := tn.NewChannel("test")
	alpha.Join(test)
	beta.Join(test)

	alpha.SetChannelMode(test, "+q", beta)
	beta.SetChannelMode(test, "-q+v", alpha, alpha)

	tn.ExpectAll(test.Member(alpha).IsOwner().Not())
	tn.ExpectAll(test.Member(alpha).IsVoice())
	tn.ExpectAll(test.Member(beta).IsOwner())

	tn.Shutdown()
	wg.Wait()
}

func TestNetworkChannel_FarQuit(t *testing.T) {
	var wg sync.WaitGroup
	tn, hubA := newTestNetwork(t, "hub.a", &wg)
	hubC := hubA.NewLink("hub.b").NewLink("hub.c")

	alpha := hubA.NewClient("alpha")
	beta := hubC.NewClient("beta")

	test := tn.NewChannel("test")
	alpha.Join(test)
	beta.Join(test)

	beta.Quit("Bye!")

	tn.ExpectAll(beta.Exists().Not())
	tn.ExpectAll(test.Member(beta).Exists().Not())

	tn.Shutdown()
	wg.Wait()
}
