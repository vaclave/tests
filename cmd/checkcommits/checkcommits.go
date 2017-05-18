// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/urfave/cli"
)

// CommitConfig encapsulates the user configuration options, but is also
// used to pass some state between functions (FoundFixes).
type CommitConfig struct {
	// set when a fixes #XXX" commit is found
	FoundFixes bool

	// All commits must have a sign-off
	NeedSOBS bool

	// Atleast one commit must specify a bug that it fixes.
	NeedFixes bool

	MaxSubjectLineLength int
	MaxBodyLineLength    int

	SobString   string
	FixesString string

	FixesPattern *regexp.Regexp
	SobPattern   *regexp.Regexp
}

const (
	defaultSobString   = "Signed-off-by"
	defaultFixesString = "Fixes"

	defaultMaxSubjectLineLength = 75
	defaultMaxBodyLineLength    = 72

	defaultCommit = "HEAD"
	defaultBranch = "master"
)

var (
	// Full path to git(1) command
	gitPath = ""
	verbose = false
	debug   = false

	errNoCommit = errors.New("Need commit")
	errNoBranch = errors.New("Need branch")
	errNoConfig = errors.New("Need config")
)

func init() {
	var err error
	gitPath, err = exec.LookPath("git")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot find git in PATH\n")
		os.Exit(1)
	}
}

func checkCommitSubject(config *CommitConfig, commit, subject string) error {
	if config == nil {
		return errNoConfig
	}

	if commit == "" {
		return fmt.Errorf("Commit not specified")
	}

	if subject == "" {
		return fmt.Errorf("Commit %v: empty subject", commit)
	}

	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("Commit %v: pure whitespace subject", commit)
	}

	subsystemPattern := regexp.MustCompile(`^[^ ][^ ]*.*:`)
	matches := subsystemPattern.FindStringSubmatch(subject)
	if matches == nil {
		return fmt.Errorf("Commit %v: Failed to find subsystem in subject: %q",
			commit, subject)
	}

	length := len(subject)
	if length > config.MaxSubjectLineLength {
		return fmt.Errorf("commit %v: subject too long (max %v, got %v): %q",
			commit, config.MaxSubjectLineLength, length, subject)
	}

	if config.FixesString != "" && config.FixesPattern != nil {
		matches = config.FixesPattern.FindStringSubmatch(subject)

		if matches != nil {
			config.FoundFixes = true
		}
	}

	return nil
}

func checkCommitBodyLine(config *CommitConfig, commit string, line string,
	lineNum int, nonWhitespaceOnlyLine *int,
	sobPattern *regexp.Regexp, sobLine *int) error {
	if config == nil {
		return errNoConfig
	}

	if line == "" {
		return nil
	}

	// Remove all whitespace
	trimmedLine := strings.TrimSpace(line)

	if *nonWhitespaceOnlyLine == -1 {
		if trimmedLine != "" {
			*nonWhitespaceOnlyLine = lineNum
		}
	}

	// Check first character of line. If it's _not_
	// alphabetic, length limits don't apply.
	rune, _ := utf8.DecodeRune([]byte{line[0]})

	if !unicode.IsLetter(rune) {
		return nil
	}

	fixesMatches := config.FixesPattern.FindStringSubmatch(line)
	if fixesMatches != nil {
		config.FoundFixes = true
	}

	sobMatch := sobPattern.FindStringSubmatch(line)
	if sobMatch != nil {
		*sobLine = lineNum
	}

	// Note: SOB lines are *NOT* checked for max line
	// length: it isn't reasonable to penalise someone
	// people with long names ;)
	if *sobLine != -1 {
		return nil
	}

	// If the line comprises of only a single word, it may be
	// something like a URL (it's certainly very unlikely to be a
	// normal word if the default lengths are being used), so length
	// checks won't be applied to it.
	singleWordLine := false
	if trimmedLine == line {
		singleWordLine = true
	}

	length := len(line)
	if length > config.MaxBodyLineLength && !singleWordLine {
		return fmt.Errorf("commit %v: body line %d too long (max %v, got %v): %q",
			commit, 1+lineNum, config.MaxBodyLineLength, length, line)
	}

	return nil
}

func checkCommitBody(config *CommitConfig, commit string, body []string) error {
	if config == nil {
		return errNoConfig
	}

	if commit == "" {
		return fmt.Errorf("Commit not specified")
	}

	if body == nil {
		return fmt.Errorf("Commit %v: empty body", commit)
	}

	// note that sign-off lines must start in the first column
	sobPattern := regexp.MustCompile(fmt.Sprintf("^%s:", config.SobString))

	// line number which contains a sign-off line.
	sobLine := -1

	// line number containing only whitespace
	nonWhitespaceOnlyLine := -1

	for i, line := range body {
		err := checkCommitBodyLine(config, commit, line, i,
			&nonWhitespaceOnlyLine, sobPattern, &sobLine)
		if err != nil {
			return err
		}
	}

	if nonWhitespaceOnlyLine == -1 {
		return fmt.Errorf("Commit %v: pure whitespace body", commit)
	}

	if config.NeedSOBS && sobLine == -1 {
		return fmt.Errorf("Commit %v: no %v specified", commit, config.SobString)
	}

	if sobLine == nonWhitespaceOnlyLine {
		return fmt.Errorf("Commit %v: single-line %q body not permitted", commit, config.SobString)
	}

	return nil
}

func getCommitRange(commit, branch string) ([]string, error) {
	if commit == "" {
		return nil, errNoCommit
	}

	if branch == "" {
		return nil, errNoBranch
	}

	var args []string

	args = append(args, gitPath)
	args = append(args, "rev-list")
	args = append(args, "--no-merges")
	args = append(args, "--reverse")
	args = append(args, fmt.Sprintf("%s..%s", branch, commit))

	return runCommand(args)
}

func getCommitSubject(commit string) (string, error) {
	if commit == "" {
		return "", errNoCommit
	}

	var args []string

	args = append(args, gitPath)
	args = append(args, "log")
	args = append(args, "-1")
	args = append(args, "--pretty=%s")
	args = append(args, commit)

	lines, err := runCommand(args)
	if err != nil {
		return "", err
	}

	return lines[0], nil
}

func getCommitBody(commit string) ([]string, error) {
	if commit == "" {
		return []string{}, errNoCommit
	}

	var args []string

	args = append(args, gitPath)
	args = append(args, "log")
	args = append(args, "-1")
	args = append(args, "--pretty=%b")
	args = append(args, commit)

	return runCommand(args)
}

func checkCommitFull(config *CommitConfig, commit, subject string, body []string) error {
	if config == nil {
		return errNoConfig
	}

	if commit == "" {
		return errNoCommit
	}

	if subject == "" {
		return fmt.Errorf("Commit %v: empty subject", commit)
	}

	if body == nil {
		return fmt.Errorf("Commit %v: empty body", commit)
	}

	err := checkCommitSubject(config, commit, subject)
	if err != nil {
		return err
	}

	err = checkCommitBody(config, commit, body)
	return err
}

func checkCommit(config *CommitConfig, commit string) error {
	if config == nil {
		return errNoConfig
	}

	if commit == "" {
		return errNoCommit
	}

	subject, err := getCommitSubject(commit)
	if err != nil {
		return err
	}

	body, err := getCommitBody(commit)
	if err != nil {
		return err
	}

	return checkCommitFull(config, commit, subject, body)
}

// checkCommits performs checks on specified list of commits
func checkCommits(config *CommitConfig, commits []string) error {
	if config == nil {
		return errNoConfig
	}

	if commits == nil {
		return errNoCommit
	}

	config.FixesPattern = regexp.MustCompile(fmt.Sprintf("%s:* *#\\d+", config.FixesString))

	for _, commit := range commits {
		if verbose {
			fmt.Printf("Checking commit %s\n", commit)
		}
		err := checkCommit(config, commit)
		if err != nil {
			return err
		}
	}

	if config.NeedFixes && !config.FoundFixes {
		return fmt.Errorf("No %q found", config.FixesString)
	}

	return nil
}

func detectCIEnvironment() (commit, branch string) {
	var name string

	if os.Getenv("TRAVIS") != "" {
		name = "TravisCI"

		commit = os.Getenv("TRAVIS_COMMIT")
		branch = os.Getenv("TRAVIS_BRANCH")

	} else if os.Getenv("SEMAPHORE") != "" {
		name = "SemaphoreCI"

		commit = os.Getenv("REVISION")
		branch = os.Getenv("BRANCH_NAME")
	}

	if verbose && name != "" {
		fmt.Printf("Detected %v Environment\n", name)
	}

	return commit, branch
}

// preChecks performs checks on the range of commits described by commit
// and branch.
func preChecks(config *CommitConfig, commit, branch string) error {
	if config == nil {
		return errNoConfig
	}

	if commit == "" {
		return errNoCommit
	}

	if branch == "" {
		return errNoBranch
	}

	commits, err := getCommitRange(commit, branch)
	if err != nil {
		return err
	}

	if verbose {
		l := len(commits)
		extra := ""
		if l != 1 {
			extra = "s"
		}
		fmt.Printf("Found %d commit%s between commit %v and branch %v\n",
			l, extra, commit, branch)
	}

	return checkCommits(config, commits)
}

// runCommand runs the command specified by args and returns its stdout
// lines as a slice.
func runCommand(args []string) (stdout []string, err error) {
	var outBytes, errBytes bytes.Buffer

	cmd := exec.Command(args[0], args[1:]...)

	cmdline := strings.Join(args, " ")
	if debug {
		fmt.Printf("Running: %q\n", cmdline)
	}

	cmd.Stdout = &outBytes
	cmd.Stderr = &errBytes

	err = cmd.Run()
	if err != nil {
		e := fmt.Errorf("Failed to run command %v: %v"+
			" (stdout: %v, stderr: %v)",
			cmdline, err, outBytes.String(), errBytes.String())
		return nil, e
	}

	lines := strings.Split(outBytes.String(), "\n")

	// Remove last line if empty
	length := len(lines)
	last := lines[length-1]
	if last == "" {
		lines = lines[:length-1]
	}

	return lines, nil
}

// NewCommitConfig creates a new CommitConfig object.
func NewCommitConfig(needFixes, needSignOffs bool, fixesPrefix, signoffPrefix string, bodyLength, subjectLength int) *CommitConfig {
	config := &CommitConfig{
		NeedSOBS:             needSignOffs,
		NeedFixes:            needFixes,
		MaxBodyLineLength:    bodyLength,
		MaxSubjectLineLength: subjectLength,
		SobString:            defaultSobString,
		FixesString:          defaultFixesString,
	}

	if config.MaxBodyLineLength == 0 {
		config.MaxBodyLineLength = defaultMaxBodyLineLength
	}

	if config.MaxSubjectLineLength == 0 {
		config.MaxSubjectLineLength = defaultMaxSubjectLineLength
	}

	if fixesPrefix != "" {
		config.FixesString = fixesPrefix
	}

	if signoffPrefix != "" {
		config.SobString = signoffPrefix
	}

	return config
}

// ignoreBranch returns true if branch is specified in the slice.
func ignoreBranch(branch string, branches []string) (bool, error) {
	if branch == "" {
		return false, errNoBranch
	}

	for _, b := range branches {
		if b == branch {
			return true, nil
		}
	}

	return false, nil
}

// getCommitAndBranch determines the commit and branch to use.
func getCommitAndBranch(c *cli.Context) (commit, branch string, err error) {
	count := c.NArg()
	if count == 0 {
		// no arguments so check the environment
		commit, branch = detectCIEnvironment()
	}

	if count > 2 {
		return "", "", errors.New("Too many arguments. Run with '--help' for usage")
	}

	if commit == "" && count >= 1 {
		commit = c.Args().Get(0)
	}

	if branch == "" && count == 2 {
		branch = c.Args().Get(1)
	}

	if commit == "" {
		commit = defaultCommit

		if verbose {
			fmt.Printf("Defaulting commit to %s\n", commit)
		}
	}

	if branch == "" {
		branch = defaultBranch

		if verbose {
			fmt.Printf("Defaulting branch to %s\n", branch)
		}
	}

	ignore, err := ignoreBranch(branch, c.StringSlice("ignore-branch"))
	if err != nil {
		return "", "", err
	}

	if ignore {
		if verbose {
			fmt.Printf("Exiting as ignored branch %q found.\n", branch)
		}

		os.Exit(0)
	}

	return commit, branch, nil
}

func main() {
	app := cli.NewApp()
	app.Name = "commitchecks"
	app.Version = "0.0.1"
	app.Description = "perform checks on git commits"
	app.Usage = app.Description
	app.UsageText = fmt.Sprintf("%s [global options] [commit [branch]]\n", app.Name)
	app.UsageText += fmt.Sprintf("\n")
	app.UsageText += fmt.Sprintf("   If not specified, commit and branch will be set automatically\n")
	app.UsageText += fmt.Sprintf("   from standard CI environment variables.\n")
	app.UsageText += fmt.Sprintf("   If not running under a recognised CI environment (Travis or Sempahore),\n")
	app.UsageText += fmt.Sprintf("   commit will default to %q and branch to %q.", defaultCommit, defaultBranch)

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "need-fixes, f",
			Usage: fmt.Sprintf("Ensure atleast one commit has a %q entry", defaultFixesString),
		},

		cli.BoolFlag{
			Name:  "need-sign-offs, s",
			Usage: fmt.Sprintf("Ensure all commits have a %q entry", defaultSobString),
		},

		cli.BoolFlag{
			Name:        "verbose",
			Usage:       "Display informational messages",
			EnvVar:      "CHECKCOMMITS_VERBOSE",
			Destination: &verbose,
		},

		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Display debug messages (implies verbose)",
			EnvVar:      "CHECKCOMMITS_DEBUG",
			Destination: &debug,
		},

		cli.StringFlag{
			Name:  "fixes-prefix",
			Usage: fmt.Sprintf("Fixes prefix used as an alternative to %q", defaultFixesString),
		},

		cli.StringFlag{
			Name:  "sign-off-prefix",
			Usage: fmt.Sprintf("Sign-off prefix used as an alternative to %q", defaultSobString),
		},

		cli.StringSliceFlag{
			Name:  "ignore-branch",
			Usage: "branch to ignore (can be specified multiple times)",
		},

		cli.UintFlag{
			Name:  "body-length",
			Usage: "Specify maximum body line length",
			Value: uint(defaultMaxBodyLineLength),
		},

		cli.UintFlag{
			Name:  "subject-length",
			Usage: "Specify maximum subject line length",
			Value: uint(defaultMaxSubjectLineLength),
		},
	}

	app.Action = func(c *cli.Context) error {
		if c.Bool("debug") {
			verbose = true
		}

		commit, branch, err := getCommitAndBranch(c)
		if err != nil {
			return err
		}

		config := NewCommitConfig(c.Bool("need-fixes"),
			c.Bool("need-sign-offs"),
			c.String("fixes-prefix"),
			c.String("sign-off-prefix"),
			int(c.Uint("body-length")),
			int(c.Uint("subject-length")))

		return preChecks(config, commit, branch)
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}