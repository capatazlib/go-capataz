package api

import "time"

// Error represents an API error
type Error struct {
	Error string `json:"error"`
}

// Plan represents a saboteur plan
type Plan struct {
	Name        string        `json:"name"`
	SubtreeName string        `json:"subreeName"`
	Duration    time.Duration `json:"duration"`
	Period      time.Duration `json:"period"`
	Attempts    uint32        `json:"attempts"`
}

// Plans represents a list of saboteur plans
type Plans struct {
	Plans []Plan `json:"plans"`
}
