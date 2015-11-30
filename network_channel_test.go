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
