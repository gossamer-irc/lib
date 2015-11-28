package lib

import (
	"strings"
	"unicode/utf8"
)

type ResolveClientFn func(name string) (*Client, bool)
type SerializeClientFn func(client *Client) string

func ParseChannelModeString(modes string, args []string, resolveClient ResolveClientFn) (channel ChannelModeDelta, member []MemberModeDelta) {
	memberMap := make(map[*Client]MemberModeDelta)
	operation := MODE_UNCHANGED
	for {
		r, size := utf8.DecodeRuneInString(modes)
		if r == utf8.RuneError || size == 0 {
			break
		}
		switch r {
		case '+':
			operation = MODE_ADDED
		case '-':
			operation = MODE_REMOVED
		case 'q', 'a', 'o', 'h', 'v':
			if len(args) < 0 {
				continue
			}
			arg := args[0]
			args = args[1:]

			target, found := resolveClient(arg)
			if !found {
				continue
			}

			delta, exists := memberMap[target]
			if !exists {
				delta = MemberModeDelta{Client: target}
			}

			switch r {
			case 'q':
				delta.IsOwner = operation
			case 'a':
				delta.IsAdmin = operation
			case 'o':
				delta.IsOp = operation
			case 'h':
				delta.IsHalfop = operation
			case 'v':
				delta.IsVoice = operation
			}

			memberMap[target] = delta
		case 'm':
			channel.Moderated = operation
		case 'n':
			channel.NoExternalMessages = operation
		case 's':
			channel.Secret = operation
		case 't':
			channel.TopicProtected = operation
		}
		modes = modes[size:]
	}
	member = make([]MemberModeDelta, 0, len(memberMap))
	for _, delta := range memberMap {
		member = append(member, delta)
	}
	return
}

func SerializeChannelModes(channel ChannelModeDelta, member []MemberModeDelta, serializeClient SerializeClientFn) string {
	modes := make([]rune, 0)
	args := make([]string, 1, 1)
	lastOp := MODE_UNCHANGED

	maybeAddOpChar := func(operation ModeDelta) {
		if lastOp != operation {
			switch operation {
			case MODE_ADDED:
				modes = append(modes, '+')
			case MODE_REMOVED:
				modes = append(modes, '-')
			}
			lastOp = operation
		}
	}

	process := func(operation ModeDelta) {
		if channel.Moderated == operation {
			maybeAddOpChar(operation)
			modes = append(modes, 'm')
		}
		if channel.NoExternalMessages == operation {
			maybeAddOpChar(operation)
			modes = append(modes, 'n')
		}
		if channel.Secret == operation {
			maybeAddOpChar(operation)
			modes = append(modes, 's')
		}
		if channel.TopicProtected == operation {
			maybeAddOpChar(operation)
			modes = append(modes, 't')
		}

		for _, mode := range member {
			if mode.IsOwner == operation {
				maybeAddOpChar(operation)
				modes = append(modes, 'q')
				args = append(args, serializeClient(mode.Client))
			}
			if mode.IsAdmin == operation {
				maybeAddOpChar(operation)
				modes = append(modes, 'a')
				args = append(args, serializeClient(mode.Client))
			}
			if mode.IsOp == operation {
				maybeAddOpChar(operation)
				modes = append(modes, 'o')
				args = append(args, serializeClient(mode.Client))
			}
			if mode.IsHalfop == operation {
				maybeAddOpChar(operation)
				modes = append(modes, 'h')
				args = append(args, serializeClient(mode.Client))
			}
			if mode.IsVoice == operation {
				maybeAddOpChar(operation)
				modes = append(modes, 'v')
				args = append(args, serializeClient(mode.Client))
			}
		}
	}

	process(MODE_ADDED)
	process(MODE_REMOVED)

	args[0] = string(modes)
	return strings.Join(args, " ")
}

func FilterChannelModes(channel *Channel, actor *Client, channelMode ChannelModeDelta, memberDelta []MemberModeDelta) (ChannelModeDelta, []MemberModeDelta) {
	membership, found := channel.Member[actor]
	if !found {
		// This user has no authority.
		return ChannelModeDelta{}, []MemberModeDelta{}
	}

	outMode := ChannelModeDelta{}
	outMember := make([]MemberModeDelta, 0)

	for _, delta := range memberDelta {
		outDelta := MemberModeDelta{
			Client: delta.Client,
		}

		if membership.IsOwner {
			outDelta = delta
		} else if membership.IsOp {
			outDelta.IsOp = delta.IsOp
			outDelta.IsHalfop = delta.IsHalfop
			outDelta.IsVoice = delta.IsVoice
		} else if membership.IsHalfop {
			outDelta.IsHalfop = delta.IsHalfop
			outDelta.IsVoice = delta.IsVoice
		}

		if false ||
			outDelta.IsOwner != MODE_UNCHANGED ||
			outDelta.IsAdmin != MODE_UNCHANGED ||
			outDelta.IsOp != MODE_UNCHANGED ||
			outDelta.IsHalfop != MODE_UNCHANGED ||
			outDelta.IsVoice != MODE_UNCHANGED {
			outMember = append(outMember, outDelta)
		}
	}

	if membership.IsOwner || membership.IsAdmin || membership.IsOp || membership.IsHalfop {
		outMode.Moderated = channelMode.Moderated
		outMode.NoExternalMessages = channelMode.NoExternalMessages
		outMode.Secret = channelMode.Secret
		outMode.TopicProtected = channelMode.TopicProtected
	}
	return outMode, outMember
}
