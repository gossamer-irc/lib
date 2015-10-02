package lib

type Subnet struct {
	Name string

	Client map[string]*Client
	Channel map[string]*Channel
}

func NewSubnet(name string) *Subnet {
	return &Subnet {
		Name: name,
		Client: make(map[string]*Client),
		Channel: make(map[string]*Channel),
	}
}
