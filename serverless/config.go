package serverless

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// validateConfiguration rejects invalid upstreams so forwarding cannot form loops.
func validateConfiguration(servers []Server, frequency *float64) (err error) {
	if len(servers) < 1 || len(servers) > 6 {
		return errors.New("choose between 1 and 6 DNS servers")
	}
	if frequency != nil && (math.IsNaN(*frequency) || math.IsInf(*frequency, 0) || *frequency < 0 || *frequency > 8) {
		return errors.New("regular DNS frequency must be between 0 and 8 queries per second")
	}
	for i, server := range servers {
		if server.ID != fmt.Sprintf("dns-%d", i+1) {
			return errors.New("server IDs must be sequential")
		}
		if server.ForwardTo != "" && (!server.Poisoned || !slices.ContainsFunc(servers, func(upstream Server) bool { return upstream.ID == server.ForwardTo && !upstream.Poisoned })) {
			return errors.New("only exfil servers may forward, and their upstream must be a normal DNS server")
		}
	}
	return
}
