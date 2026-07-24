// server.go: struct Server dung chung va middleware cho tat ca endpoint dashboard-api.
package dashboardapi

import "github.com/lethingochan27925/hivemind/pkg/cockroach"

type Server struct {
	DB *cockroach.Client
}

func NewServer(db *cockroach.Client) *Server {
	return &Server{DB: db}
}
