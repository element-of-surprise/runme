// Package config holds our basic translation from a TOML configuration file to a usable struct.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/silas/dag"
)

// Config holds our configuration from the configuration file.
type Config struct {
	// Runners are all the Runner(s) defined in our DAG.
	Runners []Runner

	m map[string]Runner
}

// Root returns the root node.
func (c *Config) Root() Runner {
	return c.Runners[0]
}

// Map returns the Runners with their names as keys in a map.
func (c *Config) Map() map[string]Runner {
	return c.m
}

// validate validates all the Runners.
func (c *Config) validate() error {
	if len(c.Runners) == 0 {
		return fmt.Errorf("no valid Runners defined")
	}

	runners := make(map[string]Runner, len(c.Runners))
	g := &dag.AcyclicGraph{}
	for _, r := range c.Runners {
		if err := r.validate(runners); err != nil {
			return err
		}
		g.Add(r.Name)
	}

	for _, r := range runners {
		if _, ok := runners[r.Next]; !ok {
			return fmt.Errorf("Runner(%s) had Next(%s) which doesn't exist", r.Name, r.Next)
		}
		g.Connect(dag.BasicEdge(r.Name, r.Next))
	}

	if err := g.Validate(); err != nil {
		return fmt.Errorf("config represents a non-valid DAG graph: %s", err)
	}

	c.m = runners

	return nil
}

// Runner represents a runner node in the DAG.
type Runner struct {
	// Name is the name of this Runner. (Required)
	Name string
	// Cmd is the command to execute. You may use {{.KeyName}} for value substitution that comes from the passed
	// map. All "\n" and "\" characters are turned into spaces before parsing. (Required)
	Cmd string
	// Sleep indicates the amount of time to sleep before executing this command.
	Sleep time.Duration
	// Next is the name of the next Runner to execute. If there is nothing afterwards, this should be set to "END". (Required)
	Next string
	// Retries is the number of retries to attempt if this fails. Failure is marked with any non-0 return code.
	Retries int
	// RetrySleep is the time to sleep between retries.
	RetrySleep time.Duration
	// ValueKey is the unique key to store the STDOUT of this command in. This value will have TrimSpace() called on it
	// before it is stored.
	ValueKey string
}

func (r *Runner) validate(seen map[string]Runner) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("a Runner cannot have an empty name field")
	}

	r.Cmd = strings.TrimSpace(r.Cmd)
	if r.Cmd == "" {
		return fmt.Errorf("Runner(%s) had an empty Cmd field", r.Name)
	}

	r.Cmd = strings.Replace(r.Cmd, "\n", " ", -1)
	r.Cmd = strings.Replace(r.Cmd, `\`, " ", -1)

	r.Next = strings.TrimSpace(r.Next)
	if r.Next == "" {
		return fmt.Errorf("Runner(%s) has an empty Next field", r.Name)
	}

	if _, ok := seen[r.Name]; ok {
		return fmt.Errorf("Runner(%s) was defined multiple times", r.Name)
	}
	seen[r.Name] = *r

	if r.Sleep > 30*time.Minute {
		return fmt.Errorf("Runner(%s) had a Sleep time of %s which exceeds the 30 minute maximum", r.Name, r.Sleep)
	}
	if r.Retries > 100 || r.Retries < 0 {
		return fmt.Errorf("Runner(%s) had a Retries setting of %d, which exceeds the 100 maximum or is less than 0", r.Name, r.Retries)
	}
	retrySleepMax := time.Duration(r.Retries) * r.RetrySleep
	if retrySleepMax > 30*time.Minute {
		return fmt.Errorf("Runner(%s) had a Retries + RetrySleep that could take %s, which exceeds our 30 minute limit", r.Name, retrySleepMax)
	}
	return nil
}

// FromFile returns a Config from a file "p" in filesystem "fsys". This validates all the runners are correct, that all nodes referenced
// are present and validates that we have a valid DAG.
func FromFile(fsys fs.FS, p string) (*Config, error) {
	b, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if _, err := toml.Decode(string(b), c); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}
