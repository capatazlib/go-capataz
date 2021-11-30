package saboteur

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/capatazlib/go-capataz/cap"
	"github.com/capatazlib/go-capataz/saboteur/api"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// NewServer create a new saboteur HTTP management server
func NewServer(ll logrus.FieldLogger, db *sabotageDB) *Server {
	server := &Server{
		ll: ll,
		db: db,
	}

	return server
}

// NewHTTPHandler creates a `http.Handler` with endpoints that access the
// sabotage management system.
func (s *Server) NewHTTPHandler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/nodes", s.listNodes).Methods("GET")
	r.HandleFunc("/plans", s.listPlans).Methods("GET")
	r.HandleFunc("/plans", s.insertPlan).Methods("POST")
	r.HandleFunc("/plans/{name}", s.removePlan).Methods("DELETE")
	r.HandleFunc("/plans/{name}/start", s.startPlan).Methods("POST")
	r.HandleFunc("/plans/{name}/stop", s.stopPlan).Methods("POST")
	return r
}

// NewHTTPNode builds a `cap.Node` that runs the given HTTP server.
func (s *Server) NewHTTPNode(
	server *http.Server,
	certFilepath, keyFilepath string,
	opts ...cap.WorkerOpt,
) (cap.Node, error) {
	if server.Addr == "" {
		return nil, errors.New("invalid input: server's Address is empty")
	}

	if server.Handler != nil {
		return nil, errors.New("invalid input: server's http.Handler is already initialized")
	}

	handler := s.NewHTTPHandler()
	server.Handler = handler

	spec := cap.NewSupervisorSpec(
		"http",
		// Node order matters, server-shutdown should execute first on
		// termination logic.
		cap.WithNodes(
			cap.NewWorker("server", func(context.Context) error {
				// Run the server with an HTTPs configuration
				if certFilepath != "" && keyFilepath != "" {
					return server.ListenAndServeTLS(certFilepath, keyFilepath)
				}
				// Run the server without HTTPs
				return server.ListenAndServe()
			}),
			cap.NewWorker("server-shutdown", func(ctx context.Context) error {
				<-ctx.Done()
				return server.Shutdown(ctx)
			}),
		),
	)

	node := cap.Subtree(spec, opts...)

	return node, nil
}

func handleError(resp http.ResponseWriter, err error, code int) {
	data, _ := json.Marshal(api.Error{Error: err.Error()})
	resp.Header().Set("Content-Type", "application/json")
	http.Error(resp, string(data), code)
}

func (s *Server) listNodes(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	nodes, err := s.db.ListNodes(ctx)
	if err != nil {
		handleError(response, err, http.StatusInternalServerError)
		return
	}
	ns := api.Nodes{
		Nodes: make([]api.Node, 0, len(nodes)),
	}
	for _, n := range nodes {
		ns.Nodes = append(ns.Nodes, api.Node{Name: n})
	}
	data, err := json.Marshal(ns)
	if err != nil {
		handleError(response, err, http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		s.ll.WithError(err).Warn("ListNodes: failed to write headers to client")
	}
}

func (s *Server) listPlans(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	plans, err := s.db.ListPlans(ctx)
	if err != nil {
		handleError(response, err, http.StatusInternalServerError)
		return
	}
	ps := api.Plans{
		Plans: make([]api.Plan, 0, len(plans)),
	}
	for _, p := range plans {
		ps.Plans = append(ps.Plans,
			api.Plan{
				Name:        p.name,
				SubtreeName: p.subtreeName,
				Duration:    p.duration,
				Period:      p.period,
				Attempts:    uint32(p.maxAttempts),
				Running:     p.running,
			},
		)
	}
	data, err := json.Marshal(ps)
	if err != nil {
		handleError(response, err, http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		s.ll.WithError(err).Warn("ListPlans: failed to write headers to client")
	}
}

func (s *Server) insertPlan(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	p := api.Plan{}
	err = json.NewDecoder(request.Body).Decode(&p)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	err = s.db.InsertPlan(
		ctx,
		planName(p.Name),
		nodeName(p.SubtreeName),
		p.Duration,
		p.Period,
		p.Attempts,
	)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		s.ll.WithError(err).Warn("InsertPlan: failed to write headers to client")
	}
}

func (s *Server) removePlan(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	vars := mux.Vars(request)
	name := vars["name"]

	err = s.db.RemovePlan(
		ctx,
		planName(name),
	)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		s.ll.WithError(err).Warn("RemovePlan: failed to write headers to client")
	}
}

func (s *Server) startPlan(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	vars := mux.Vars(request)
	name := vars["name"]

	err = s.db.StartPlan(
		ctx,
		planName(name),
	)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		s.ll.WithError(err).Warn("StartPlan: failed to write headers to client")
	}
}

func (s *Server) stopPlan(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	vars := mux.Vars(request)
	name := vars["name"]

	err = s.db.StopPlan(
		ctx,
		planName(name),
	)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		s.ll.WithError(err).Warn("StopPlan: failed to write headers to client")
	}
}
