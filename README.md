# libgossamer

[![Build Status](https://drone.io/github.com/gossamer-irc/lib/status.png)](https://drone.io/github.com/gossamer-irc/lib/latest)

libgossamer is a Go library for the Gossamer IRC server system. Servers on a Gossamer network use this library to maintain network state, communicate via the Gossamer server to server (s2s) protocol with other servers, and expose clients using its APIs. For example, the Gossamer daemon acts as a traditional ircd, accepting client connections and interfacing them to the network. The Gossamer services, on the other hand, expose a set of pseudoclients to the network such as NickServ, ChanServ, etc using the same library.

## Difference with traditional IRC networks

Gossamer networks are different than traditional IRC networks. Not only is the s2s protocol incompatible, Gossamer defines additional concepts that have no analogue in other IRC systems. The most important of these is the *realm*.

### Realms

Conventional IRC networks share a single namespace for user nicknames and channel names. This has a number of benefits, including the property that users can belong to disparate channels and participate in them all. However, a single namespace has significant drawbacks as well. Nicknames must be unique across the network, and new users might find their preferred nicknames already in use.

This problem is compounded by the fact that many of IRC’s newer, less experienced users come to the protocol as part of “third-party” communities - forums, subreddits, hobby sites, etc - with the intention of joining a chatroom specific to their community. They already have established identities in these “realms”, including nicknames. The disassociation between their existing identity and their IRC identity is frequently lost on them, and can be a source of frustration if their favored nickname is already taken.

Ideally, new users should be able to keep their existing identity when joining the channel(s) of an existing community, but still be able to explore and venture into other channels and communities on the network. This is the motivation behind Gossamer’s realms.

Gossamer divides the traditional IRC network into separate namespaces, called *realms*. Users and channels each belong to a specific realm, shown as a colon-separated prefix. For example, the user “test” in the realm “dev” has an effective nickname of “dev:test”, and the channel “help” in the same realm is effectively “#dev:help”.

Forcing every user to carry a prefix just adds clutter, though, so Gossamer employs a trick to hide the prefix in most cases: any two clients in the same realm will see each other without the prefix, while two clients in different realms will each observe the other with a fully identified nickname.  This way, a user in a given realm can explore other channels and interact with the members there, while still maintaining an identity that’s associated with their realm.

It is also possible for a user, with the assistance of network services, to switch between realms, which generates a notification to other users of an effective nickname change. Services, however, should be able to manage a user with multiple identities (from different realms) and apply permissions accordingly.

### Zero Configuration

Gossamer employs a zero configuration system for server setup and linking, meaning there is no traditional *ircd.conf* file to write. Instead, a few command line options for servers will bootstrap them to connect to the network, where a more extensive configuration can be retrieved from a central configuration authority.

To achieve a trusted link between servers without configuration, a Gossamer network includes a certificate authority (CA) which signs TLS certificates for each server in the network. When two of these servers connect, they can verify the validity of the other side’s certificate against the network CA, and determine if the link is legitimate.

Operation works the same way - users present a signed TLS certificate when connecting, which unlocks any oper privileges they have associated with their account. This way, no oper passwords must be configured in advance.

### Binary Protocol

When linked with each other, Gossamer servers communicate using a binary protocol based on Go’s gob encoding. This allows much faster, more CPU friendly communication between servers, at the expense of human readability.
