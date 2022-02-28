// Package exec provides an Executor type for executing a series of commands that
// represent command line programs. This allows the creation of single purpose binaries
// instead of the multipurpose runme binary program.
package exec

import (
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/element-of-surprise/runme/config"
	"github.com/element-of-surprise/runme/internal/cmd"
	gfs "github.com/gopherfs/fs"
)

// ReadWriter is a file system with ReadFile() and WriteFile().
type ReadWriter interface {
	fs.ReadFileFS
	gfs.Writer
}

// Executor executes a series of command line arguments.
type Executor struct {
	// StartAt indicates you wish to start at a particular runner with this name.
	startAt string
	// FS is the filesystem that we read and write to.
	fs ReadWriter
	// Vals is the map of values that gets passed to our Sequence(s).
	vals map[string]string

	seqs []*config.Sequence

	failedNode string
}

// New creates a new Executor.
func New(startAt string, fs ReadWriter, vals map[string]string) (*Executor, error) {
	if fs == nil {
		return nil, fmt.Errorf("must pass a valid ReadWriter")
	}
	if vals == nil {
		return nil, fmt.Errorf("must pass a valid vals map")
	}
	return &Executor{startAt: startAt, fs: fs, vals: vals}, nil
}

type sequencer interface {
	Sequence() string
}

// Run runs the commands help in "c" and uses "vals" to do substiution for template arguments.
func (e *Executor) Run(c *config.Config, vals map[string]string) error {
	startAt := -1
	if e.startAt == "" {
		startAt = 0
	} else {
		for i, seq := range e.seqs {
			if e.startAt == seq.Item().(sequencer).Sequence() {
				startAt = i
			}
		}
		if startAt == -1 {
			return fmt.Errorf("couldn't find the node to start at(%s)", e.startAt)
		}
	}

	for _, node := range e.seqs[startAt:] {
		if err := e.run(node); err != nil {
			e.failedNode = node.Item().(sequencer).Sequence()
			return err
		}
	}
	return nil
}

// FailedNode is the node that was run and failed. This is an empty string if no node failed.
func (e *Executor) FailedNode() string {
	return e.failedNode
}

func (e *Executor) run(r *config.Sequence) error {
	switch v := r.Item().(type) {
	case config.CreateVar:
		if err := v.Exec(e.fs, e.vals); err != nil {
			return err
		}
	case config.WriteFile:
		if err := v.Exec(e.fs, e.vals); err != nil {
			return err
		}
	case config.Runner:
		c, err := cmd.New(v.Cmd, e.vals)
		if err != nil {
			return err
		}
		if v.Sleep.Duration > 0 {
			fmt.Println("Sleeping for: ", v.Sleep.Duration)
		}
		var b []byte
		for i := 0; i < v.Retries+1; i++ {
			if i > 0 {
				time.Sleep(v.RetrySleep.Duration)
			}
			b, err = c.Run()
			if err != nil {
				continue
			}
		}
		if err != nil {
			return err
		}
		if v.ValueKey != "" {
			e.vals[v.ValueKey] = strings.TrimSpace(string(b))
		}
	}
	return nil
}
