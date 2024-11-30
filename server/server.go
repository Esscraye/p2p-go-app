package server

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Server struct {
	peers map[string]string
	mu    sync.Mutex
}

func NewServer() *Server {
	return &Server{
		peers: make(map[string]string),
	}
}

func (s *Server) RegisterPeer(w http.ResponseWriter, r *http.Request) {
	var peer struct {
		Addr string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peer.Addr] = peer.Addr
	w.WriteHeader(http.StatusOK)
}

func (s *Server) RemovePeer(w http.ResponseWriter, r *http.Request) {
	var peer struct {
		Addr string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, peer.Addr)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) GetPeers(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	json.NewEncoder(w).Encode(s.peers)
}

func (s *Server) Start() {
	http.HandleFunc("/register-peer", s.RegisterPeer)
	http.HandleFunc("/remove-peer", s.RemovePeer)
	http.HandleFunc("/get-peers", s.GetPeers)
	http.ListenAndServe(":8080", nil)
}
