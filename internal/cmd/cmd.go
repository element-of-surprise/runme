package cmd

import (
	"log"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/element-of-surprise/runme/internal/parser"
)

// Cmd is a wrapper for exec.Cmd to allow for more elegant construction for the
// purposes of this package.
type Cmd struct {
	cmd   *exec.Cmd
	args []string
	debug bool
}

// New creates a Cmd out of the string "s" with value substitutions from vals.
func New(s string, vals map[string]string) (*Cmd, error) {
	p := parser.Line{}
	args, err := p.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("command(%s) could not be parsed: %s", s, err)
	}
	b := strings.Builder{}
	for i, arg := range args[1:] {
		b.Reset()
		tmpl, err := template.New("").Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("arg(%s) violated a text/template rule: %s\n template looks like:\n%s", arg, err, strings.Join(args, " "))
		}
		if err := tmpl.Execute(&b, vals); err != nil {
			return nil, fmt.Errorf("problem with template execution: %s", err)
		}
		s := b.String()

		// Okay.... shell uses quotes of either ' or " to wrap a mutli-space arguement as a single arugement. But exec.Cmd doesn't, so you need
		// to remove outer quotes, if they exist.
		if strings.HasPrefix(s, `"`) {
			s = strings.TrimPrefix(s, `"`)
			s = strings.TrimSuffix(s, `"`)
		}else if strings.HasPrefix(s, `'`){
			s = strings.TrimPrefix(s, `'`)
			s = strings.TrimSuffix(s, `'`)
		}

		args[i+1] = s
	}

	log.Printf("args: %#+v", args)
	c := &Cmd{
		cmd:   exec.Command(args[0], args[1:]...),
		args: args,
		debug: true,
	}
	return c.BaseEnv(), nil
}

func (c *Cmd) String() string {
	return strings.Join(c.args, " ")
}

// Debug if set to on will send the command stdout and stderr to the os.Stdout and os.Stderr.
// Defaults to true.
func (c *Cmd) Debug(on bool) *Cmd {
	c.debug = on

	return c
}

// Env allows appending to the underling exec.Cmd.Env value.
func (c *Cmd) Env(env ...string) *Cmd {
	c.cmd.Env = append(c.cmd.Env, env...)
	return c
}

// BaseEnv replaces the current exec.Cmd.Env and sets up GOPATH, HOME, and PATH.
func (c *Cmd) BaseEnv() *Cmd {
	c.cmd.Env = []string{
		"GOPATH=" + os.Getenv("GOPATH"),
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
	}
	return c
}

// Run executes the command
func (c *Cmd) Run() ([]byte, error) {
	buff := &bytes.Buffer{}
	if c.debug {
		c.cmd.Stderr = io.MultiWriter(buff, os.Stderr)
		c.cmd.Stdout = io.MultiWriter(buff, os.Stdout)
	} else {
		c.cmd.Stderr = buff
		c.cmd.Stdout = buff
	}
	if err := c.cmd.Run(); err != nil {
		return buff.Bytes(), err
	}
	return buff.Bytes(), nil
}

// Exec returns the underlying *exec.Cmd.
func (c *Cmd) Exec() *exec.Cmd {
	return c.cmd
}
