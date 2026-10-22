package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/havspect/project-times/internal/client"
	"github.com/havspect/project-times/internal/config"
)

func login(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error in one line
	urlFlag := fs.String("url", "", "server URL")
	email := fs.String("email", "", "email")
	password := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "times login: %v\n", err)
		return 2
	}

	cfg, err := config.LoadClient(*urlFlag, "", os.Getenv)
	if err != nil {
		fmt.Fprintf(stderr, "times login: %v\n", err)
		return 1
	}
	if cfg.URL == "" {
		return loginUsage(stderr)
	}

	in := bufio.NewReader(os.Stdin)
	if *email == "" {
		fmt.Fprint(stdout, "email: ")
		if *email, err = readLine(in); err != nil {
			fmt.Fprintf(stderr, "times login: email: %v\n", err)
			return 1
		}
	}
	if *password == "" {
		// shortcut: the password prompt echoes what is typed, upgrade to x/term when it is worth a dependency
		fmt.Fprint(stdout, "password: ")
		if *password, err = readLine(in); err != nil {
			fmt.Fprintf(stderr, "times login: password: %v\n", err)
			return 1
		}
	}

	c := client.New(client.Config{URL: cfg.URL})
	me, err := c.Login(*email, *password)
	if err != nil {
		fmt.Fprintf(stderr, "times login: %v\n", err)
		return 1
	}
	tok, err := c.CreateToken("times login")
	if err != nil {
		fmt.Fprintf(stderr, "times login: %v\n", err)
		return 1
	}
	cfg.Token = tok.Token
	if err := config.SaveClient(cfg); err != nil {
		fmt.Fprintf(stderr, "times login: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "logged in as %s <%s>\n", me.Name, me.Email)
	return 0
}

func loginUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: times login --url URL [--email x] [--password y]")
	return 2
}

// readLine returns one line without its terminator; EOF with bytes still counts.
func readLine(in *bufio.Reader) (string, error) {
	s, err := in.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}
