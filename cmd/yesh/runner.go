package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type stage struct {
	argv          []string
	input, output string
	append        bool
}

const maxPipelineStages = 32

func (r *runtime) expandWord(token commandToken, line int) ([]string, error) {
	if token.literal {
		return []string{token.text}, nil
	}
	return r.expand(token.text, line)
}

func validateStages(stages []stage) error {
	if len(stages) > maxPipelineStages {
		return fmt.Errorf("pipeline exceeds %d stages", maxPipelineStages)
	}
	for i, s := range stages {
		if len(s.argv) == 0 || s.argv[0] == "" {
			return fmt.Errorf("empty command")
		}
		for _, arg := range s.argv {
			if strings.ContainsRune(arg, 0) {
				return fmt.Errorf("NUL in command argument")
			}
		}
		if i != 0 && s.input != "" {
			return fmt.Errorf("input redirection is only allowed on first stage")
		}
		if i != len(stages)-1 && s.output != "" {
			return fmt.Errorf("output redirection is only allowed on last stage")
		}
		name := strings.ToLower(filepath.Base(strings.ReplaceAll(s.argv[0], `\`, "/")))
		name = strings.TrimSuffix(name, ".exe")
		if name == "exit" {
			return fmt.Errorf("exit is a script statement; use standalone exit code, not a command or pipeline stage")
		}
		// This is a narrow, documented execution policy, not an isolation
		// boundary. The runtime's safety guarantees do not depend on this list.
		switch name {
		case "sh", "bash", "dash", "ash", "zsh", "fish", "ksh", "csh", "tcsh", "cmd", "powershell", "pwsh":
			return fmt.Errorf("nested shell execution is disabled: %s", s.argv[0])
		}
		if len(stages) > 1 {
			switch name {
			case "env", "sudo", "doas", "su", "xargs", "command", "exec", "eval", "source", ".", "nohup", "setsid", "nice", "ionice", "chrt", "timeout", "busybox", "flock", "watch", "chroot", "nsenter", "unshare", "runuser", "setpriv", "systemd-run":
				return fmt.Errorf("execution wrapper is disabled in pipeline: %s", s.argv[0])
			}
			if name == "perl" || name == "ruby" || name == "node" || name == "nodejs" || name == "php" || name == "lua" || name == "luajit" || name == "python" || strings.HasPrefix(name, "python2") || strings.HasPrefix(name, "python3") {
				return fmt.Errorf("code interpreter is disabled in pipeline: %s", s.argv[0])
			}
		}
	}
	return nil
}

func (r *runtime) parseCommand(text string, line int) ([]stage, error) {
	tokens, err := words(text)
	if err != nil {
		return nil, fmt.Errorf("%s:%d: %w", r.filename, line, err)
	}
	if err := validateWords(tokens, r.filename, line); err != nil {
		return nil, err
	}
	var stages []stage
	current := stage{}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.operator && token.text == "|" {
			if len(current.argv) == 0 {
				return nil, fmt.Errorf("%s:%d: empty pipeline stage", r.filename, line)
			}
			stages = append(stages, current)
			current = stage{}
			continue
		}
		if token.operator && (token.text == ">" || token.text == ">>" || token.text == "<") {
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("%s:%d: redirection needs a file", r.filename, line)
			}
			if tokens[i+1].operator {
				return nil, fmt.Errorf("%s:%d: redirection needs a file", r.filename, line)
			}
			name, err := r.expandWord(tokens[i+1], line)
			if err != nil {
				return nil, err
			}
			if len(name) != 1 || name[0] == "" || strings.ContainsRune(name[0], 0) {
				return nil, fmt.Errorf("%s:%d: redirection target must be one value", r.filename, line)
			}
			if token.text == "<" {
				if current.input != "" {
					return nil, fmt.Errorf("%s:%d: repeated input redirection", r.filename, line)
				}
				current.input = name[0]
			} else {
				if current.output != "" {
					return nil, fmt.Errorf("%s:%d: repeated output redirection", r.filename, line)
				}
				current.output = name[0]
				current.append = token.text == ">>"
			}
			i++
			continue
		}
		args, err := r.expandWord(token, line)
		if err != nil {
			return nil, err
		}
		current.argv = append(current.argv, args...)
	}
	if len(current.argv) == 0 {
		return nil, fmt.Errorf("%s:%d: empty command", r.filename, line)
	}
	stages = append(stages, current)
	if err := validateStages(stages); err != nil {
		return nil, fmt.Errorf("%s:%d: %w", r.filename, line, err)
	}
	return stages, nil
}

func closeFiles(files ...[]*os.File) {
	for _, group := range files {
		for _, f := range group {
			if f != nil {
				_ = f.Close()
			}
		}
	}
}

func (r *runtime) formatPipeline(stages []stage) string {
	secretValues := make(map[string]bool)
	for _, v := range r.values {
		if v.secret && v.scalar != "" {
			secretValues[v.scalar] = true
		}
	}
	var out strings.Builder
	for i, s := range stages {
		if i > 0 {
			out.WriteString(" | ")
		}
		display := append([]string(nil), s.argv...)
		for i, arg := range display {
			if secretValues[arg] {
				display[i] = "<environment value>"
			}
		}
		fmt.Fprintf(&out, "%q", display)
		if s.input != "" {
			fmt.Fprintf(&out, " < %q", s.input)
		}
		if s.output != "" {
			op := ">"
			if s.append {
				op = ">>"
			}
			fmt.Fprintf(&out, " %s %q", op, s.output)
		}
	}
	return out.String()
}

func (r *runtime) run(text string, line int, fatal bool) error {
	return r.runWithin(text, line, fatal, commandTimeout)
}

func (r *runtime) runWithin(text string, line int, fatal bool, timeout time.Duration) error {
	stages, err := r.parseCommand(text, line)
	if err != nil {
		return err
	}
	if r.plan {
		fmt.Fprintf(os.Stdout, "PLAN %s\n", r.formatPipeline(stages))
		return nil
	}
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return fmt.Errorf("%s:%d: command cancelled: %w", r.filename, line, err)
	}
	for _, s := range stages {
		if _, err := exec.LookPath(s.argv[0]); err != nil {
			// Standalone builtins do not require an executable in PATH.
			if len(stages) == 1 && s.input == "" && s.output == "" && (s.argv[0] == "echo" || s.argv[0] == "true" || s.argv[0] == "false") {
				continue
			}
			return fmt.Errorf("%s:%d: resolve %s: %w", r.filename, line, s.argv[0], err)
		}
	}
	if len(stages) == 1 && stages[0].input == "" && stages[0].output == "" {
		switch stages[0].argv[0] {
		case "echo":
			fmt.Fprintln(os.Stdout, strings.Join(stages[0].argv[1:], " "))
			return nil
		case "true":
			return nil
		case "false":
			if fatal {
				return fmt.Errorf("%s:%d: command failed: false", r.filename, line)
			}
			return statusError{code: 1}
		}
	}

	deadline, stopDeadline := context.WithTimeout(parent, timeout)
	defer stopDeadline()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()
	cmds := make([]*exec.Cmd, len(stages))
	files := make([]*os.File, 0, 2)
	pipeFiles := make([]*os.File, 0, 2*(len(stages)-1))
	defer func() { closeFiles(files, pipeFiles) }()
	group := &processGroup{}
	escalationDone := make(chan struct{})
	escalationExited := make(chan struct{})
	defer func() { close(escalationDone); <-escalationExited }()
	go func() {
		defer close(escalationExited)
		select {
		case <-ctx.Done():
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = group.kill()
			case <-escalationDone:
			}
		case <-escalationDone:
		}
	}()
	for i, s := range stages {
		cmds[i] = exec.CommandContext(ctx, s.argv[0], s.argv[1:]...)
		cmds[i].Cancel = group.terminate
		cmds[i].WaitDelay = 2 * time.Second
		cmds[i].Stderr = os.Stderr
		if i == 0 {
			cmds[i].Stdin = os.Stdin
		}
		if i == 0 && s.input != "" {
			f, e := openRedirect(s.input, false, false)
			if e != nil {
				return fmt.Errorf("%s:%d: open input: %w", r.filename, line, e)
			}
			cmds[i].Stdin = f
			files = append(files, f)
		}
		if i == len(stages)-1 {
			cmds[i].Stdout = os.Stdout
		}
		if s.output != "" && i == len(stages)-1 {
			f, e := openRedirect(s.output, true, s.append)
			if e != nil {
				return fmt.Errorf("%s:%d: open output: %w", r.filename, line, e)
			}
			cmds[i].Stdout = f
			files = append(files, f)
			if len(files) > 1 {
				in, e1 := files[0].Stat()
				out, e2 := f.Stat()
				if e1 != nil || e2 != nil {
					return fmt.Errorf("%s:%d: cannot stat redirection files", r.filename, line)
				}
				if os.SameFile(in, out) && out.Mode().IsRegular() {
					return fmt.Errorf("%s:%d: input and output are the same file", r.filename, line)
				}
			}
		}
	}
	// Resolve every executable before modifying output or starting any stage.
	for _, cmd := range cmds {
		if cmd.Err != nil {
			return fmt.Errorf("%s:%d: resolve %s: %w", r.filename, line, cmd.Args[0], cmd.Err)
		}
	}
	if s := stages[len(stages)-1]; s.output != "" && !s.append {
		f := cmds[len(cmds)-1].Stdout.(*os.File)
		info, e := f.Stat()
		if e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			if e := f.Truncate(0); e != nil {
				return e
			}
		}
	}
	for i := 0; i < len(stages)-1; i++ {
		reader, writer, e := os.Pipe()
		if e != nil {
			return fmt.Errorf("%s:%d: create pipeline: %w", r.filename, line, e)
		}
		cmds[i].Stdout = writer
		cmds[i+1].Stdin = reader
		pipeFiles = append(pipeFiles, reader, writer)
	}

	started := make([]*exec.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		// Configure immediately before Start. On POSIX the first started
		// process becomes the group leader; later stages join that group.
		group.configure(cmd)
		if err := cmd.Start(); err != nil {
			_ = group.kill()
			for _, running := range started {
				_ = running.Wait()
			}
			return fmt.Errorf("%s:%d: start %s: %w", r.filename, line, cmd.Args[0], err)
		}
		group.started(cmd)
		started = append(started, cmd)
	}
	closeFiles(pipeFiles)
	pipeFiles = nil

	results := make(chan error, len(cmds))
	for _, cmd := range cmds {
		go func(cmd *exec.Cmd) { results <- cmd.Wait() }(cmd)
	}
	var last error
	for range cmds {
		if e := <-results; e != nil && last == nil {
			last = e
			cancel()
		}
	}
	if ctx.Err() != nil {
		_ = group.kill()
	}
	if deadline.Err() != nil {
		return fmt.Errorf("%s:%d: command interrupted: %w", r.filename, line, deadline.Err())
	}
	if last != nil && fatal {
		return fmt.Errorf("%s:%d: command failed: %s", r.filename, line, text)
	}
	return last
}
