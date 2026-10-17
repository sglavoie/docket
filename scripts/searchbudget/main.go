// searchbudget checks input-to-frame benchmark samples against recorded budgets.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type budget struct {
	bytesPerCycle float64
	nsPerKey      float64
}

// Each benchmark cycle types and deletes seven characters (14 rendered keys).
// Allocation ceilings leave headroom over the measured baseline. Timing targets
// are opt-in gates because shared CI machines cannot promise a stable CPU share.
var budgets = map[string]budget{
	"BenchmarkLiveSearchTyping/tasks=10000/projects=20/notes=true":   {96e6, 25e6},
	"BenchmarkLiveSearchContent/tasks=1000/query=release":            {48e6, 20e6},
	"BenchmarkLiveSearchContent/tasks=1000/query=handoff":            {64e6, 40e6},
	"BenchmarkLiveSearchContent/tasks=1000/query=zzzzzzz":            {144e6, 30e6},
	"BenchmarkLiveSearchContent/tasks=10000/query=release":           {140e6, 25e6},
	"BenchmarkLiveSearchContent/tasks=10000/query=handoff":           {280e6, 30e6},
	"BenchmarkLiveSearchContent/tasks=10000/query=zzzzzzz":           {1050e6, 45e6},
	"BenchmarkLiveSearchColdUniqueContent/tasks=10000/query=release": {220e6, 50e6},
	"BenchmarkLiveSearchColdUniqueContent/tasks=10000/query=handoff": {440e6, 110e6},
	"BenchmarkLiveSearchColdUniqueContent/tasks=10000/query=zzzzzzz": {1150e6, 80e6},
}

var cpuSuffix = regexp.MustCompile(`-\d+$`)

type sample struct{ bytes, ns float64 }

func check(input io.Reader, output io.Writer, strictTime bool) error {
	measurements := make(map[string][]sample)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		name := cpuSuffix.ReplaceAllString(fields[0], "")
		if _, known := budgets[name]; !known {
			continue
		}
		var s sample
		for i := 2; i+1 < len(fields); i += 2 {
			value, err := strconv.ParseFloat(fields[i], 64)
			if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("invalid benchmark measurement for %s", name)
			}
			switch fields[i+1] {
			case "B/op":
				s.bytes = value
			case "ns/key":
				s.ns = value
			}
		}
		if s.bytes == 0 || s.ns == 0 {
			return fmt.Errorf("missing B/op or ns/key for %s", name)
		}
		measurements[name] = append(measurements[name], s)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	var failures []string
	names := make([]string, 0, len(budgets))
	for name := range budgets {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		limit := budgets[name]
		samples := measurements[name]
		if len(samples) < 3 {
			failures = append(failures, name+": need at least three samples")
			continue
		}
		bytes, times := make([]float64, len(samples)), make([]float64, len(samples))
		for i, s := range samples {
			bytes[i], times[i] = s.bytes, s.ns
		}
		slices.Sort(bytes)
		slices.Sort(times)
		allocated, elapsed := bytes[len(bytes)/2], times[len(times)/2]
		if _, err := fmt.Fprintf(output, "%s: median %.1f MB/cycle (budget %.0f), %.1f ms/key (target %.0f)\n", name, allocated/1e6, limit.bytesPerCycle/1e6, elapsed/1e6, limit.nsPerKey/1e6); err != nil {
			return err
		}
		if allocated > limit.bytesPerCycle {
			failures = append(failures, name+": allocation budget exceeded")
		}
		if elapsed > limit.nsPerKey {
			if strictTime {
				failures = append(failures, name+": latency target exceeded")
			} else {
				if _, err := fmt.Fprintln(output, "  Latency above target; rerun on an idle machine with --strict-time."); err != nil {
					return err
				}
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "\n"))
	}
	return nil
}

func main() {
	strict := flag.Bool("strict-time", false, "also fail when median latency exceeds its target")
	flag.Parse()
	if err := check(os.Stdin, os.Stdout, *strict); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
