// Package config holds our basic translation from a TOML configuration file to a usable struct.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	gfs "github.com/gopherfs/fs"
	//"github.com/silas/dag"
)

// Config holds our configuration from the configuration file.
type Config struct {
	// Required are required values that must be passed in.
	Required []Required
	// CreateVars are a list of variables to create. This operation is done before any
	// sequence has run, but it does allow use of variables stored in the vals map.
	CreateVars []CreateVar
	// Sequences is a sequence of actions to execute, in order.
	//Seqs []Sequence

	Seqs []toml.Primitive

	sequences []*Sequence
	required  map[string]*regexp.Regexp
}

// Root returns the root node.
func (c *Config) Root() *Sequence {
	return c.sequences[0]
}

func (c *Config) Sequences() []*Sequence {
	return c.sequences
}

// validate validates all the Runners.
func (c *Config) validate(fsys gfs.Writer, vals map[string]string) error {
	if len(c.sequences) == 0 {
		return fmt.Errorf("no valid Sequences defined")
	}

	c.required = make(map[string]*regexp.Regexp, len(c.Required))
	for _, req := range c.Required {
		if _, ok := c.required[req.Name]; ok {
			return fmt.Errorf("a Required field(%s) was set twice", req.Name)
		}
		var re *regexp.Regexp
		var err error
		if req.Regex == "" {
			c.required[req.Name] = nil
			continue
		}
		re, err = regexp.Compile(req.Regex)
		if err != nil {
			return fmt.Errorf("a Required field(%s) had an invalid regex: %s", req.Name, req.Regex)
		}
		c.required[req.Name] = re
	}

	if len(c.required) != len(vals) {
		return fmt.Errorf("there are %d values required, but only saw %d passed", len(c.required), len(vals))
	}

	for k, v := range vals {
		re, ok := c.required[k]
		if !ok {
			return fmt.Errorf("value passed with key(%s) that was not found in config.Required", k)
		}
		if re == nil {
			continue
		}
		if !re.MatchString(v) {
			return fmt.Errorf("value passed with key(%s) did not have a valid value(%s)", k, v)
		}
	}

	seen := map[string]bool{}

	for _, v := range c.CreateVars {
		if err := v.validate(seen); err != nil {
			return err
		}
		if err := v.Exec(fsys, vals); err != nil {
			return err
		}
	}

	runners := 0

	for i, seq := range c.sequences {
		switch v := seq.Item().(type) {
		case CreateVar:
			if err := v.validate(seen); err != nil {
				return err
			}
			c.sequences[i] = &Sequence{createVar: v}
		case Runner:
			runners++
			if err := v.validate(seen); err != nil {
				return err
			}
			c.sequences[i] = &Sequence{runner: v}
		case WriteFile:
			if err := v.validate(seen); err != nil {
				return err
			}
			c.sequences[i] = &Sequence{writeFile: v}
		default:
			return fmt.Errorf("Sequence is a type(%T) that is not recognized: ", seq)
		}
	}
	if runners == 0 {
		return fmt.Errorf("no Sequence was defined as a Runner")
	}
	return nil
}

// Required is a required value that must be passed in before anything is executed.
type Required struct {
	// Name is the name of the value that must be passed.
	Name string
	// Regex is the regexp.Regexp that must match for the value to be valid.
	// If not set, the value is not checked.
	Regex string
}

// Sequence represents a sequenced action to perform. This is either a CreateVar, Runner or WriteFile.
type Sequence struct {
	createVar CreateVar
	runner    Runner
	writeFile WriteFile
}

func (s *Sequence) Item() interface{} {
	if !reflect.ValueOf(s.createVar).IsZero() {
		return s.createVar
	}
	if !reflect.ValueOf(s.runner).IsZero() {
		return s.runner
	}
	if !reflect.ValueOf(s.writeFile).IsZero() {
		return s.writeFile
	}
	return nil
}

// CreateVar creates a variable.
type CreateVar struct {
	// Name is the unique name of the CreateVar sequence.
	Name string
	// Key is the to save the variable in.
	Key string
	// Value is the value to save. This can contain template variables that reference keys
	// stored in our val map.
	Value string
}

func (c *CreateVar) Sequence() string {
	return c.Name
}

func (c *CreateVar) validate(seen map[string]bool) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("a CreateVar cannot have an empty name field")
	}
	if _, ok := seen[c.Name]; ok {
		return fmt.Errorf("CreateVar(%s) was defined multiple times", c.Name)
	}
	seen[c.Name] = true

	if strings.TrimSpace(c.Key) != c.Key {
		return fmt.Errorf("CreateVar cannot have key(%s): has leading or trailing space", c.Key)
	}
	return nil
}

func (c CreateVar) Exec(fsys fs.FS, vals map[string]string) error {
	tmpl, err := template.New("").Parse(c.Value)
	if err != nil {
		return fmt.Errorf("CreateVar(%s) violated a text/template rule: %s", c.Key, err)
	}
	b := strings.Builder{}
	if err := tmpl.Execute(&b, vals); err != nil {
		return fmt.Errorf("CreateVar(%s): problem with template execution: %s", c.Key, err)
	}
	vals[c.Key] = b.String()
	return nil
}

// WriteFile writes a file to disk.
type WriteFile struct {
	// Name is the unique name of the CreateVar sequence.
	Name string
	// Path is where to store the file.
	Path string
	// Value is the value to write to the file. This can contain template variables that reference keys
	// stored in our val map.
	Value string
}

func (w *WriteFile) Sequence() string {
	return w.Name
}

func (w *WriteFile) validate(seen map[string]bool) error {
	w.Name = strings.TrimSpace(w.Name)
	if w.Name == "" {
		return errors.New("a WriteFile cannot have an empty name field")
	}
	if _, ok := seen[w.Name]; ok {
		return fmt.Errorf("WriteFile(%s) was defined multiple times", w.Name)
	}
	seen[w.Name] = true

	if strings.TrimSpace(w.Path) == "" {
		return fmt.Errorf("cannot have an empty path")
	}
	w.Value = strings.TrimSpace(w.Value)
	if w.Value == "" {
		return fmt.Errorf("cannot write an empty file")
	}
	return nil
}

func (w WriteFile) Exec(wr gfs.Writer, vals map[string]string) error {
	tmpl, err := template.New("").Parse(w.Value)
	if err != nil {
		return fmt.Errorf("WriteFile(%s) violated a text/template rule: %s", w.Path, err)
	}
	b := bytes.Buffer{}
	if err := tmpl.Execute(&b, vals); err != nil {
		return fmt.Errorf("WriteFile(%s): problem with template execution: %s", w.Path, err)
	}

	if err := wr.WriteFile(w.Path, b.Bytes(), 0600); err != nil {
		return fmt.Errorf("WriteFile(%s): %s", w.Path, err)
	}
	return nil
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// Runner represents a runner node in the DAG.
type Runner struct {
	// Name is the name of this Runner. (Required)
	Name string
	// Cmd is the command to execute. You may use {{.KeyName}} for value substitution that comes from the passed
	// map. All "\n" and "\" characters are turned into spaces before parsing. (Required)
	Cmd string
	// Sleep indicates the amount of time to sleep before executing this command.
	Sleep duration
	// Retries is the number of retries to attempt if this fails. Failure is marked with any non-0 return code.
	Retries int
	// RetrySleep is the time to sleep between retries.
	RetrySleep duration
	// ValueKey is the unique key to store the STDOUT of this command in. This value will have TrimSpace() called on it
	// before it is stored.
	ValueKey string
}

func (r *Runner) Sequence() string {
	return r.Name
}

func (r *Runner) validate(seen map[string]bool) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("a Runner cannot have an empty name field")
	}

	r.Cmd = strings.TrimSpace(r.Cmd)
	if r.Cmd == "" {
		return fmt.Errorf("Runner(%s) had an empty Cmd field", r.Name)
	}
	lines := strings.Split(r.Cmd, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
		lines[i] = strings.TrimPrefix(lines[i], "\t")
		lines[i] = strings.TrimPrefix(lines[i], `\t`)
		lines[i] = strings.TrimSuffix(lines[i], `\`)
	}
	r.Cmd = strings.Join(lines, " ")

	if _, ok := seen[r.Name]; ok {
		return fmt.Errorf("Runner(%s) was defined multiple times", r.Name)
	}
	seen[r.Name] = true

	if r.Sleep.Duration > 30*time.Minute {
		return fmt.Errorf("Runner(%s) had a Sleep time of %s which exceeds the 30 minute maximum", r.Name, r.Sleep)
	}
	if r.Retries > 100 || r.Retries < 0 {
		return fmt.Errorf("Runner(%s) had a Retries setting of %d, which exceeds the 100 maximum or is less than 0", r.Name, r.Retries)
	}
	retrySleepMax := duration{time.Duration(r.Retries) * r.RetrySleep.Duration}
	if retrySleepMax.Duration > 30*time.Minute {
		return fmt.Errorf("Runner(%s) had a Retries + RetrySleep that could take %s, which exceeds our 30 minute limit", r.Name, retrySleepMax)
	}
	return nil
}

// FromFile returns a Config from a file "p" in filesystem "fsys". This validates all the runners are correct, that all nodes referenced
// are present and validates that we have a valid DAG.
func FromFile(fsys gfs.Writer, p string, vals map[string]string) (*Config, error) {
	b, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	md, err := toml.Decode(string(b), c)
	if err != nil {
		return nil, err
	}

	for i, seq := range c.Seqs {
		s := &Sequence{}
		if err := md.PrimitiveDecode(seq, &s.createVar); err == nil && s.Item() != nil {
			// PrimitiveDecode will decode a Runner into a CreateVar because both have
			// the Attribute "Name". You can't get a metadata for just this to check if the keys
			// have been decoded. So, we check that a unique and required attribute is set.
			if s.createVar.Key != "" {
				c.sequences = append(c.sequences, s)
				continue
			}
			// Reset it to the zero value because it was a bad decode.
			s.createVar = CreateVar{}
		}
		if err := md.PrimitiveDecode(seq, &s.runner); err == nil && s.Item() != nil {
			if s.runner.Cmd != "" {
				c.sequences = append(c.sequences, s)
				continue
			}
			s.runner = Runner{}
		}
		if err := md.PrimitiveDecode(seq, &s.writeFile); err == nil && s.Item() != nil {
			if s.writeFile.Path != "" {
				c.sequences = append(c.sequences, s)
				continue
			}
			s.writeFile = WriteFile{}
		}
		return nil, fmt.Errorf("Sequence(%d) does not seem to decode into anything", i)
	}

	if err := c.validate(fsys, vals); err != nil {
		return nil, err
	}

	if len(md.Undecoded()) > 0 {
		keys := []string{}
		for _, tk := range md.Undecoded() {
			keys = append(keys, tk.String())
		}
		return nil, fmt.Errorf("had unknown keys: %s", strings.Join(keys, ", "))
	}

	return c, nil
}
