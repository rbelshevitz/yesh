package main

import (
	"fmt"
	"strings"
	"time"
)

const serviceResourceStateSchema = 1

const maxServiceResourceStateSamples = 600

// serviceResourceState is a completed, bounded measurement record. It is a
// data format only: reading it never recreates a command or a process action.
type serviceResourceState struct {
	Schema            int                          `json:"schema"`
	Kind              string                       `json:"kind"`
	Manager           string                       `json:"manager"`
	Unit              string                       `json:"unit"`
	Cgroup            string                       `json:"cgroup"`
	InvocationID      string                       `json:"invocation_id,omitempty"`
	StartedAt         time.Time                    `json:"started_at"`
	FinishedAt        time.Time                    `json:"finished_at"`
	Samples           []serviceResourceStateSample `json:"samples"`
	Complete          bool                         `json:"complete"`
	IncompleteReasons []string                     `json:"incomplete_reasons,omitempty"`
	Summary           map[string]string            `json:"summary,omitempty"`
}

type serviceResourceStateSample struct {
	ObservedAt          time.Time                        `json:"observed_at"`
	Metrics             map[string]string                `json:"metrics"`
	ProcessCount        int                              `json:"process_count"`
	RSSBytes            uint64                           `json:"rss_bytes"`
	ProcessesIncomplete bool                             `json:"processes_incomplete"`
	Processes           []serviceResourceProcessIdentity `json:"processes,omitempty"`
}

type serviceResourceProcessIdentity struct {
	PID       int    `json:"pid"`
	StartTime uint64 `json:"start_time"`
	Name      string `json:"name,omitempty"`
	RSSBytes  uint64 `json:"rss_bytes,omitempty"`
}

func (s serviceResourceState) validate() error {
	if s.Schema != serviceResourceStateSchema {
		return fmt.Errorf("unsupported resource state schema %d", s.Schema)
	}
	if s.Kind != "service-resource-measurement" {
		return fmt.Errorf("unexpected resource state kind %q", s.Kind)
	}
	if s.Manager != "system" && s.Manager != "user" {
		return fmt.Errorf("unexpected systemd manager %q", s.Manager)
	}
	if s.Unit == "" || s.Cgroup == "" || !strings.HasPrefix(s.Cgroup, "/") {
		return fmt.Errorf("resource state has invalid unit or cgroup identity")
	}
	if s.StartedAt.IsZero() || s.FinishedAt.IsZero() || s.FinishedAt.Before(s.StartedAt) {
		return fmt.Errorf("resource state has invalid measurement interval")
	}
	if len(s.Samples) == 0 || len(s.Samples) > maxServiceResourceStateSamples {
		return fmt.Errorf("resource state has %d samples; expected 1..%d", len(s.Samples), maxServiceResourceStateSamples)
	}
	previous := time.Time{}
	for i, sample := range s.Samples {
		if sample.ObservedAt.Before(s.StartedAt) || sample.ObservedAt.After(s.FinishedAt) || (!previous.IsZero() && !sample.ObservedAt.After(previous)) {
			return fmt.Errorf("resource state sample %d is out of order or outside the interval", i)
		}
		if sample.ProcessCount < 0 {
			return fmt.Errorf("resource state sample %d has negative process count", i)
		}
		for _, process := range sample.Processes {
			if process.PID <= 0 || process.StartTime == 0 {
				return fmt.Errorf("resource state sample %d has invalid process identity", i)
			}
		}
		previous = sample.ObservedAt
	}
	return nil
}
