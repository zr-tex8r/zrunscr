package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

const (
	progName = "zrunscr"
	version  = "0.2.0"
	modDate  = "2017/08/15"
)

type extEntry struct {
	ext, cmd string
}
type ptnEntry struct {
	ptn, cmd string
}

var basePath string
var extDecl []extEntry
var ptnDecl []ptnEntry

func init() {
	basePath = os.Args[0]
	if path, err := os.Executable(); err == nil {
		basePath = path
	}
}

func main() {
	readConfig()
	spawn(commandArgs())
}

func readConfig() {
	extDecl, ptnDecl = make([]extEntry, 0, 8), make([]ptnEntry, 0, 16)
	// get config file text
	cfgPath := filepath.Join(filepath.Dir(basePath), progName+".cfg")
	cfg, err := ioutil.ReadFile(cfgPath)
	sure(err == nil, "cannot open config file: %v", cfgPath)
	// parse lines
	rxLine := regexp.MustCompile(`^(\S+)\s*=\s*(.*)`)
	for n, line := range strings.Split(string(cfg), "\n") {
		line = strings.Trim(line, " \t\r")
		if line == "" || line[0] == '#' {
			continue
		}
		match := rxLine.FindStringSubmatch(line)
		sure(match != nil, "error in config file: line %d", n+1)
		sure(len(match) == 3, "OOPS")
		name, cmd := match[1], match[2]
		if name[0] == '.' {
			extDecl = append(extDecl, extEntry{name, cmd})
		} else {
			ptnDecl = append(ptnDecl, ptnEntry{name, cmd})
		}
	}
}

func commandArgs() []string {
	dir, name := filepath.Dir(basePath), filepath.Base(os.Args[0])
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name[:len(name)-4]
	}
	// by extensions
	xname := filepath.Join(dir, name)
	for _, ed := range extDecl {
		if _, err := os.Stat(xname + ed.ext); err == nil {
			return parseCmdLine(ed.cmd, xname+ed.ext, true)
		}
	}
	// by patterns
	for _, pd := range ptnDecl {
		ptn, err := namePattern(pd.ptn)
		sure(err == nil, "malformed pattern: %v", pd.ptn)
		if ptn.MatchString(name) {
			return parseCmdLine(pd.cmd, name, false)
		}
	}
	// not found
	sure(false, "no command entry for: %v", name)
	return nil
}

func parseCmdLine(cmd, extra string, force bool) []string {
	elts := strings.Fields(cmd)
	for i, elt := range elts {
		if elt == "<>" {
			elts[i] = extra
			return append(elts, os.Args[1:]...)
		}
	}
	// no <> found
	if force {
		elts = append(elts, extra)
	}
	return append(elts, os.Args[1:]...)
}

func namePattern(pat string) (*regexp.Regexp, error) {
	// /REGEXP/ is a regexp
	if pat[0] == '/' {
		if pat[len(pat)-1] == '/' {
			pat = pat[:len(pat)-1]
		}
		return regexp.Compile(pat[1:])
	}
	// convert wildcard pattern to regexp
	buf := make([]byte, 0, len(pat)*2)
	for _, c := range pat {
		if c == '?' {
			buf = append(buf, '.')
		} else if c == '*' {
			buf = append(buf, '.', '*')
		} else {
			buf = append(buf, regexp.QuoteMeta(string(c))...)
		}
	}
	return regexp.Compile(string(buf))
}

func exitStatus(ps *os.ProcessState) int {
	if status, ok := ps.Sys().(syscall.WaitStatus); ok {
		if status.Exited() {
			return status.ExitStatus()
		}
	}
	return 128
}

func spawn(args []string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	switch err := err.(type) {
	case *exec.Error:
		fmt.Fprintf(os.Stderr, "%v: cannot spawn %v: %v\n",
			progName, err.Name, err.Error())
		os.Exit(127)
	case *exec.ExitError:
		os.Exit(exitStatus(err.ProcessState))
	}
}

func sure(ok bool, format string, args ...interface{}) {
	if !ok {
		if format == "" {
			format = "ERROR"
		}
		fmt.Fprintf(os.Stderr, progName+": "+format, args...)
		os.Exit(1)
	}
}
