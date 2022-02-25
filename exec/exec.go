// Package exec provides an Executor type for executing a series of commands that
// represent command line programs. This allows the creation of single purpose binaries
// instead of the multipurpose runme binary program.
package exec

import (
	"fmt"
	"strings"
	"time"

	"github.com/element-of-surprise/runme/config"
	"github.com/element-of-surprise/runme/internal/cmd"
)

// Executor executes a series of command line arguments.
type Executor struct {
	// StartAt indicates you wish to start at a particular runner with this name.
	StartAt string

	m    map[string]config.Runner
	vals map[string]string

	failedNode string
}

// Run runs the commands help in "c" and uses "vals" to do substiution for template arguments.
func (e *Executor) Run(c *config.Config, vals map[string]string) error {
	var node config.Runner
	if e.StartAt == "" {
		node = c.Root()
	} else {
		var ok bool
		if node, ok = e.m[e.StartAt]; !ok {
			return fmt.Errorf("couldn't find the node to start at(%s)", e.StartAt)
		}
	}

	e.m = c.Map()
	e.vals = vals

	for {
		next, err := e.run(node)
		if err != nil {
			e.failedNode = node.Name
			return err
		}

		if next.Name == "" {
			return nil
		}
		node = next
	}
	panic("should never get here")
}

// FailedNode is the node that was run and failed. This is an empty string if no node failed.
func (e *Executor) FailedNode() string {
	return e.failedNode
}

func (e *Executor) run(r config.Runner) (config.Runner, error) {
	c, err := cmd.New(r.Cmd, e.vals)
	if err != nil {
		return config.Runner{}, err
	}

	if r.Sleep > 0 {
		fmt.Println("Sleeping for: ", r.Sleep)
	}

	var b []byte

	for i := 0; i < r.Retries+1; i++ {
		if i > 0 {
			time.Sleep(r.RetrySleep)
		}
		b, err = c.Run()
		if err != nil {
			continue
		}
	}

	if err != nil {
		return config.Runner{}, err
	}

	if r.ValueKey != "" {
		e.vals[r.ValueKey] = strings.TrimSpace(string(b))
	}

	return e.m[r.Next], nil
}
