package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	logFile := os.Getenv("LOG_FILE_PATH")
	if logFile == "" {
		logFile = "logs/default.log" // fallback log file
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %s", err)
	}
	log.SetOutput(file)
}

type Peer struct {
	Addr      string            `json:"addr"`
	FileParts map[string]string `json:"file_parts"` // map[filename]fil_part
	LastCheck time.Time         `json:"last_check"`
}

type Server struct {
	peers map[string]Peer
	files map[string]map[string]string // map[filename]map[peer_addr]file_part
	mu    sync.Mutex
}

func NewServer() *Server {
	return &Server{
		peers: make(map[string]Peer),
		files: make(map[string]map[string]string),
	}
}

func (s *Server) RegisterPeer(w http.ResponseWriter, r *http.Request) {
	var peer struct {
		Addr      string            `json:"addr"`
		FileParts map[string]string `json:"file_parts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		log.Printf("Error registering peer: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peer.Addr] = Peer{Addr: peer.Addr, LastCheck: time.Now()}
	log.Printf("Peer registered: %s", peer.Addr)

	for fileName, filePart := range peer.FileParts {
		if s.files[fileName] == nil {
			s.files[fileName] = make(map[string]string)
		}
		s.files[fileName][peer.Addr] = filePart
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) RemovePeer(w http.ResponseWriter, r *http.Request) {
	var peer struct {
		Addr string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		log.Printf("Error removing peer: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for fileName := range s.files {
		delete(s.files[fileName], peer.Addr)
		if len(s.files[fileName]) == 0 {
			delete(s.files, fileName)
		}
	}
	delete(s.peers, peer.Addr)
	log.Printf("Peer removed: %s", peer.Addr)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) GetPeers(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkPeerStatus()
	json.NewEncoder(w).Encode(s.peers)
	json.NewEncoder(w).Encode(s.files)
	log.Println("Peers list requested")
}

func (s *Server) checkPeerStatus() {
	for addr, peer := range s.peers {
		if time.Since(peer.LastCheck) > 5*time.Minute {
			delete(s.peers, addr)
			log.Printf("Peer timed out: %s", addr)
		}
	}
}

func (s *Server) UpdatePeerFileParts(w http.ResponseWriter, r *http.Request) {
	var peer Peer
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		log.Printf("Error updating peer file parts: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for fileName, filePart := range peer.FileParts {
		if s.files[fileName] == nil {
			s.files[fileName] = make(map[string]string)
		}
		s.files[fileName][peer.Addr] = filePart
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) QueryFileParts(w http.ResponseWriter, r *http.Request) {
	var request struct {
		FileName string `json:"file_name"`
		Addr     string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error querying file parts: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	peersWithFileParts := s.files[request.FileName]
	response := make(map[string]string)
	for peerAddr, filePart := range peersWithFileParts {
		if peerAddr != request.Addr {
			response[peerAddr] = filePart
		}
	}
	json.NewEncoder(w).Encode(response)
	log.Printf("File parts requested: %s", request.FileName)
}

func (s *Server) Start() {
	http.HandleFunc("/register-peer", s.RegisterPeer)
	http.HandleFunc("/update-peer-file-parts", s.UpdatePeerFileParts)
	http.HandleFunc("/remove-peer", s.RemovePeer)
	http.HandleFunc("/get-peers", s.GetPeers)
	http.HandleFunc("/query-file-parts", s.QueryFileParts)
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %s", err)
	}
}
