package lib

import (
	"testing"
)

func TestParseMode_Simple(t *testing.T) {
	channel, member := ParseChannelModeString("+nt", []string{}, func(name string) (*Client, bool) {
		return nil, false
	})
	if channel.NoExternalMessages != MODE_ADDED {
		t.Error("NoExternalMessages not being added")
	}
	if channel.TopicProtected != MODE_ADDED {
		t.Error("TopicProtected not added")
	}
	if len(member) != 0 {
		t.Errorf("Expected no member mode changes, but got %d", len(member))
	}
}

func TestParseMode_ModeArg(t *testing.T) {
	client := &Client{}
	_, member := ParseChannelModeString("+nth", []string{"client"}, func(name string) (*Client, bool) {
		if name != "client" {
			t.Errorf("Expected 'client', got '%s'", name)
			return nil, false
		}
		return client, true
	})
	if len(member) != 1 {
		t.Fatalf("Expected a member mode change, but got %d", len(member))
	}
	if member[0].Client != client {
		t.Error("Expected 'client' client but didn't get it")
	}
	if member[0].IsHalfop != MODE_ADDED {
		t.Error("Expected 'halfop' to be added")
	}
}

func TestStringifyModes_Simple(t *testing.T) {
	channel := ChannelModeDelta{
		TopicProtected: MODE_ADDED,
		Moderated:      MODE_ADDED,
	}
	modeStr := StringifyChannelModes(channel, []MemberModeDelta{}, nil)
	if modeStr != "+mt" {
		t.Errorf("Expected '+mt', got '%s'", modeStr)
	}
}

func TestStringifyModes_Negative(t *testing.T) {
	channel := ChannelModeDelta{
		TopicProtected: MODE_REMOVED,
		Moderated:      MODE_ADDED,
	}
	delta := MemberModeDelta{
		Client:  &Client{},
		IsOwner: MODE_ADDED,
		IsOp:    MODE_REMOVED,
	}
	modeStr := StringifyChannelModes(channel, []MemberModeDelta{delta}, func(client *Client) string {
		if client != delta.Client {
			t.Error("Expected known client, got unknown")
			return "unknown"
		}
		return "client"
	})
	if modeStr != "+mq-to client client" {
		t.Errorf("Got unexpected mode string '%s'", modeStr)
	}
}
