package lib

import (
	"log"
)

type Server struct {
	Name string
	Desc string

	Link *Link
	Route *Server
	Hub *Server

	Links map[string]*Server
}

func NewRemoteServer(name, desc string, hub *Server) *Server {
	s := &Server{
		Name: name,
		Desc: desc,
		Hub: hub,
		Route: hub.Route,
		Links: make(map[string]*Server),
	}
	hub.Links[name] = s
	return s
}

func NewLocalServer(name, desc string, link *Link, hub *Server) *Server {
	s := &Server{
		Name: name,
		Desc: desc,
		Link: link,
		Links: make(map[string]*Server),
		Hub: hub,
	}
	s.Route = s
	return s
}

func (s *Server) Send(msg SSMessage) {
	log.Printf("[%s -> %s]: %s", s.Route.Hub.Name, s.Route.Name, msg.String())
	err := s.Route.Link.WriteMessage(msg)
	if err != nil {
		log.Fatalf("[%s -> %s] error: %v", s.Route.Hub.Name, s.Route.Name, err)
	}
}

func (s *Server) Serialize() SSMessage {
	return &SSServer {
		Name: s.Name,
		Desc: s.Desc,
		Via: s.Hub.Name,
	}
}
