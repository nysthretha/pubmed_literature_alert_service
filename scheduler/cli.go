package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

// dispatchCLI returns true if an argv subcommand was handled (and exits), or
// false to let the normal scheduler startup proceed.
func dispatchCLI() bool {
	if len(os.Args) < 2 {
		return false
	}
	switch os.Args[1] {
	case "create-admin":
		runCreateUserCmd(os.Args[2:], true)
		return true
	case "create-user":
		runCreateUserCmd(os.Args[2:], false)
		return true
	case "reset-password":
		runResetPasswordCmd(os.Args[2:])
		return true
	case "-h", "--help", "help":
		printCLIHelp()
		os.Exit(0)
		return true
	}
	return false
}

func printCLIHelp() {
	fmt.Println(`scheduler — PubMed alerts backend

Usage:
  scheduler                                   run the scheduler + HTTP server
  scheduler create-admin --username U --email E
  scheduler create-user  --username U --email E
  scheduler reset-password --username U

CLI commands prompt for passwords interactively and do NOT run migrations;
they only touch the users table.`)
}

func runCreateUserCmd(args []string, isAdmin bool) {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	var username, email string
	fs.StringVar(&username, "username", "", "username (required)")
	fs.StringVar(&email, "email", "", "email address (required)")
	_ = fs.Parse(args)
	if username == "" || email == "" {
		fmt.Fprintln(os.Stderr, "error: --username and --email are required")
		os.Exit(2)
	}

	db := openDBOrExit()
	defer db.Close()

	pwd := promptPasswordTwice()
	hash, err := auth.HashPassword(pwd)
	if err != nil {
		die("hash password", err)
	}

	user, err := auth.CreateUser(context.Background(), db, username, email, hash, isAdmin)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			fmt.Fprintln(os.Stderr, "error: a user with that username or email already exists")
			os.Exit(1)
		}
		die("create user", err)
	}
	fmt.Printf("created user id=%d username=%s is_admin=%t\n", user.ID, user.Username, user.IsAdmin)
}

func runResetPasswordCmd(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	var username string
	fs.StringVar(&username, "username", "", "username (required)")
	_ = fs.Parse(args)
	if username == "" {
		fmt.Fprintln(os.Stderr, "error: --username is required")
		os.Exit(2)
	}

	db := openDBOrExit()
	defer db.Close()

	pwd := promptPasswordTwice()
	hash, err := auth.HashPassword(pwd)
	if err != nil {
		die("hash password", err)
	}

	if err := auth.UpdatePassword(context.Background(), db, username, hash); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			fmt.Fprintf(os.Stderr, "error: no user with username %q\n", username)
			os.Exit(1)
		}
		die("update password", err)
	}
	fmt.Printf("password reset for %s\n", username)
}

// promptPasswordTwice reads a password with echo disabled on a TTY, or falls
// back to line-buffered stdin reads when invoked non-interactively (e.g. from
// a script piping credentials). Both lines are still required.
func promptPasswordTwice() string {
	read := readPassword
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		read = readLineFallback
	}

	fmt.Fprint(os.Stderr, "Password: ")
	p1 := read()
	if len(p1) < 8 {
		fmt.Fprintln(os.Stderr, "error: password must be at least 8 characters")
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "Confirm:  ")
	p2 := read()
	if p1 != p2 {
		fmt.Fprintln(os.Stderr, "error: passwords do not match")
		os.Exit(1)
	}
	return p1
}

func readPassword() string {
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		die("read password", err)
	}
	return string(b)
}

// stdinReader is package-scoped so subsequent readLineFallback calls reuse the
// same buffer — constructing a fresh bufio.Reader per call would overread and
// drop subsequent lines still sitting in the buffer.
var stdinReader = bufio.NewReader(os.Stdin)

func readLineFallback() string {
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		die("read password (stdin)", err)
	}
	return strings.TrimRight(line, "\r\n")
}

func openDBOrExit() *sql.DB {
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		fmt.Fprintln(os.Stderr, "error: POSTGRES_URL is not set")
		os.Exit(2)
	}
	db, err := openDB(pgURL)
	if err != nil {
		die("open db", err)
	}
	return db
}

func die(what string, err error) {
	slog.Error(what, "err", err)
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", what, err)
	os.Exit(1)
}
