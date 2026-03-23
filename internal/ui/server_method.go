package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"

	"reqx/internal/history"
)

// Start registers routes, opens the browser, and blocks serving.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	url := fmt.Sprintf("http://localhost%s", addr)

	sub, err := fs.Sub(FS, "assets")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/history", s.apiHistory)
	mux.HandleFunc("/api/history/{id}", s.apiHistoryDetail)
	mux.HandleFunc("/api/history/{id}/dag", s.apiHistoryDAG)


	go openBrowser(url)
	fmt.Printf("ReqX UI running at %s  (Ctrl+C to stop)\n", url)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) apiHistory(w http.ResponseWriter, _ *http.Request) {
	runs, err := s.db.ListRuns(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []history.RunRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

func (s *Server) apiHistoryDAG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing run id", http.StatusBadRequest)
		return
	}
	nodes, err := s.db.GetDAGNodes(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}


func (s *Server) apiHistoryDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing run id", http.StatusBadRequest)
		return
	}
	stats, err := s.db.GetRunStats(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []history.StatRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start()
}
