package eval

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"kittender/add/internal/core"
	"kittender/add/internal/runner"
)

// Plan says what to run: every fixture × model × seed.
type Plan struct {
	Genome   core.Genome
	Models   []string // "provider:model"
	Seeds    []int64
	Fixtures []Fixture
}

// Result pairs a session record with its score.
type Result struct {
	Fixture string        `json:"fixture"`
	Signal  string        `json:"signal,omitempty"` // receptor runs: the receptor's one signal
	Score   Score         `json:"score"`
	Record  runner.Record `json:"record"`
}

type job struct {
	idx     int
	fixture Fixture
	session runner.Session
}

// Run executes the plan with at most `parallel` sessions at a time.
// onDone is called once per finished session (from several goroutines,
// so it must be safe for concurrent use); results come back in plan order.
//
// JS analogy: a promise pool. The `jobs` channel is the queue, each
// goroutine is a worker pulling from it, and the WaitGroup is Promise.all.
func Run(ctx context.Context, r *runner.Runner, p Plan, parallel int, onDone func(Result)) []Result {
	var jobsList []job
	for _, m := range p.Models {
		for _, seed := range p.Seeds {
			g := p.Genome // structs are copied on assignment: this is a new genome value
			g.Model, g.Seed = m, seed
			g.ID = fmt.Sprintf("%s@%s", p.Genome.ID, m)
			for _, f := range p.Fixtures {
				jobsList = append(jobsList, job{
					idx:     len(jobsList),
					fixture: f,
					session: runner.Session{
						ID:     fmt.Sprintf("l0-%s-%s-s%d", f.ID, slug(m), seed),
						Genome: g,
						Bundle: f.Bundle(),
					},
				})
			}
		}
	}

	results := make([]Result, len(jobsList))
	jobs := make(chan job)
	var wg sync.WaitGroup
	if parallel < 1 {
		parallel = 1
	}
	for w := 0; w < parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				rec := r.Run(ctx, j.session)
				res := Result{Fixture: j.fixture.ID, Score: ScoreRecord(j.fixture, rec), Record: rec}
				results[j.idx] = res // each index is written by one goroutine only
				if onDone != nil {
					onDone(res)
				}
			}
		}()
	}
	for _, j := range jobsList {
		select {
		case jobs <- j:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	return results
}

func slug(s string) string {
	return strings.NewReplacer(":", "_", "/", "_", ".", "_").Replace(s)
}
