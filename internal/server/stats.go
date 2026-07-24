package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

type statsResponse struct {
	Uptime          string           `json:"uptime"`
	StartedAt       int64            `json:"started_at"`
	Requests        uint64           `json:"requests"`
	Errors          uint64           `json:"errors"`
	AvgLatencyUs    float64          `json:"avg_latency_us"`
	FloodWaits      uint64           `json:"flood_waits"`
	Auth401s        uint64           `json:"auth_401s"`
	Forbidden       uint64           `json:"forbidden"`
	BytesIn         uint64           `json:"bytes_in"`
	BytesOut        uint64           `json:"bytes_out"`
	ActiveBots      int              `json:"active_bots"`
	Methods         []methodStatOut  `json:"methods"`
	MethodsRegistered int            `json:"methods_registered"`
}

type methodStatOut struct {
	Method       string  `json:"method"`
	Count        uint64  `json:"count"`
	Errored      uint64  `json:"errored"`
	AvgLatencyUs float64 `json:"avg_latency_us"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	reqs := metrics.Requests.Load()
	total := metrics.TotalDuration.Load()
	avg := float64(0)
	if reqs > 0 {
		avg = float64(total) / float64(reqs) / 1000
	}

	methods := []methodStatOut{}
	metrics.MethodCounts.Range(func(k, v any) bool {
		name := k.(string)
		ms := v.(*methodStat)
		count := ms.count.Load()
		if count == 0 {
			return true
		}
		avgUs := float64(ms.durSum.Load()) / float64(count) / 1000
		methods = append(methods, methodStatOut{
			Method:       name,
			Count:        count,
			Errored:      ms.errored.Load(),
			AvgLatencyUs: avgUs,
		})
		return true
	})
	sort.Slice(methods, func(i, j int) bool { return methods[i].Count > methods[j].Count })

	resp := &statsResponse{
		Uptime:            time.Since(metrics.StartedAt).Round(time.Second).String(),
		StartedAt:         metrics.StartedAt.Unix(),
		Requests:          reqs,
		Errors:            metrics.Errors.Load(),
		AvgLatencyUs:      avg,
		FloodWaits:        metrics.FloodWaits.Load(),
		Auth401s:          metrics.Auth401s.Load(),
		Forbidden:         metrics.Forbidden.Load(),
		BytesIn:           metrics.BytesIn.Load(),
		BytesOut:          metrics.BytesOut.Load(),
		ActiveBots:        s.mgr.ActiveBots(),
		Methods:           methods,
		MethodsRegistered: len(handlers),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}
