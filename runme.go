package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/element-of-surprise/runme/config"
	"github.com/element-of-surprise/runme/exec"
	"github.com/google/uuid"
	osfs "github.com/gopherfs/fs/io/os"
)

var (
	conf     = flag.String("config", "", "The TOML configuration file.")
	resume   = flag.String("resume", "", "The path to a resume file you wish to use to resume a failed run.")
	valsJSON = flag.String("vals", "", "A JSON map of map[string]string used to insert values in templates.")
)

func main() {
	flag.Parse()

	ofs, err := osfs.New()
	if err != nil {
		fmt.Printf("Error accessing OS filesystem: %s\n", err)
		os.Exit(1)
	}

	vals := map[string]string{}
	if *valsJSON != "" {
		if err := json.Unmarshal([]byte(*valsJSON), &vals); err != nil {
			fmt.Printf("Errorf unmarshalling --vals into our map: %s\n", err)
			os.Exit(1)
		}
	}

	c, err := config.FromFile(ofs, *conf, vals)
	if err != nil {
		fmt.Printf("Error opening config file(%s): %s\n", *conf, err)
		os.Exit(1)
	}

	startAt := ""
	if *resume != "" {
		b, err := fs.ReadFile(ofs, *resume)
		if err != nil {
			fmt.Printf("Error opening resume file(%s): %s\n", *resume, err)
			os.Exit(1)
		}

		r := &resumeConf{}
		if err := json.Unmarshal(b, &r); err != nil {
			fmt.Printf("Error unmarshalling resume file(%s): %s\n", *resume, err)
			os.Exit(1)
		}
		if err := r.validate(); err != nil {
			fmt.Printf("Error validating resume file(%s): %s\n", *resume, err)
			os.Exit(1)
		}

		startAt = r.StartAt
		if len(r.Vals) > 0 {
			vals = r.Vals
		}
	}

	e, err := exec.New(c.Sequences(), startAt, ofs, vals)
	if err != nil {
		panic(err)
	}

	if err := e.Run(c, vals); err != nil {
		fmt.Printf("Error: The program had a problem: %s\n", err)

		r := &resumeConf{Vals: vals, StartAt: e.FailedNode()}
		b, err := json.MarshalIndent(r, "", "\t")
		if err != nil {
			fmt.Printf("could not create a resume file: %s\n", err)
			os.Exit(1)
		}

		var p string
		if *resume == "" {
			id := uuid.New().String()
			p = filepath.Join(os.TempDir(), id+".resume.json")
		} else {
			p = filepath.Join(*resume)
		}

		if err := os.WriteFile(p, b, 0660); err != nil {
			fmt.Printf("problem writing resume file: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("your resume file ID is: %s\n", filepath.Base(p))
		os.Exit(1)
	}

	fmt.Println("program ended successfully")
}

type resumeConf struct {
	Vals    map[string]string
	StartAt string
}

func (r *resumeConf) validate() error {
	r.StartAt = strings.TrimSpace(r.StartAt)
	if r.StartAt == "" {
		return fmt.Errorf("StartAt was not set")
	}
	return nil
}
