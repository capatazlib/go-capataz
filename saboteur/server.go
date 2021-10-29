package saboteur

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"

	"github.com/capatazlib/go-capataz/saboteur/api"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// NewServer create a new saboteur HTTP management server
func NewServer(db *sabotageDB) *Server {
	return &Server{
		db: db,
	}
}

// Listen starts a HTTP server on the provided host and port
func (s *Server) Listen(host string, port string) {
	r := mux.NewRouter()
	r.HandleFunc("/nodes", s.listNodes).Methods("GET")
	r.HandleFunc("/plans", s.listPlans).Methods("GET")
	r.HandleFunc("/plans", s.insertPlan).Methods("POST")
	r.HandleFunc("/plans/{name}", s.removePlan).Methods("DELETE")
	r.HandleFunc("/plans/{name}/start", s.startPlan).Methods("POST")
	r.HandleFunc("/plans/{name}/stop", s.stopPlan).Methods("POST")
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.ServeHTTP(w, req)
	})
	http.Handle("/", handler)

	// TODO allow injecting logger
	logrus.WithFields(logrus.Fields{
		"host": host,
		"port": port,
	}).Info("saboteur HTTP server starting")

	err := http.ListenAndServe(net.JoinHostPort(host, port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleError(resp http.ResponseWriter, err error, code int) {
	data, _ := json.Marshal(api.Error{Error: err.Error()})
	resp.Header().Set("Content-Type", "application/json")
	http.Error(resp, string(data), code)
}

func (s *Server) listNodes(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	p := api.Node{}
	err = json.NewDecoder(request.Body).Decode(&p)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	nodes, err := s.db.ListNodes(ctx)
	if err != nil {
		handleError(response, err, http.StatusInternalServerError)
		return
	}
	ns := api.Nodes{
		Nodes: make([]api.Node, len(nodes)),
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
		logrus.Warn("ListNodes: failed to write headers to client", err)
	}
}

func (s *Server) listPlans(response http.ResponseWriter, request *http.Request) {
	var err error
	ctx := context.Background()

	p := api.Plan{}
	err = json.NewDecoder(request.Body).Decode(&p)
	if err != nil {
		handleError(response, err, http.StatusBadRequest)
		return
	}

	plans, err := s.db.ListPlans(ctx)
	if err != nil {
		handleError(response, err, http.StatusInternalServerError)
		return
	}
	ps := api.Plans{
		Plans: make([]api.Plan, len(plans)),
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
		logrus.Warn("ListPlans: failed to write headers to client", err)
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
		logrus.Warn("InsertPlan: failed to write headers to client", err)
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
		logrus.Warn("RemovePlan: failed to write headers to client", err)
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
		logrus.Warn("StartPlan: failed to write headers to client", err)
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
		logrus.Warn("StopPlan: failed to write headers to client", err)
	}
}
