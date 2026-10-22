package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/config"
	"github.com/havspect/project-times/internal/store"
)

func admin(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "create" {
		return adminUsage(stderr)
	}

	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error in one line
	email := fs.String("email", "", "email")
	name := fs.String("name", "", "name")
	db := fs.String("db", config.DefaultDB, "sqlite file path")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "times admin create: %v\n", err)
		return 2
	}
	if *email == "" || *name == "" {
		return adminUsage(stderr)
	}

	// admin create opens the database directly: there is no running server to authenticate against yet.
	st, err := store.Open(*db)
	if err != nil {
		fmt.Fprintf(stderr, "times admin create: open %s: %v\n", *db, err)
		return 1
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		fmt.Fprintf(stderr, "times admin create: migrate %s: %v\n", *db, err)
		return 1
	}

	if _, err := st.UserByEmail(*email); err == nil {
		fmt.Fprintf(stderr, "times admin create: %s already exists\n", *email)
		return 1
	} else if !errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "times admin create: %v\n", err)
		return 1
	}

	password, err := auth.GeneratePassword()
	if err != nil {
		fmt.Fprintf(stderr, "times admin create: generate password: %v\n", err)
		return 1
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(stderr, "times admin create: hash password: %v\n", err)
		return 1
	}

	u := &store.User{
		Email:        *email,
		Name:         *name,
		PasswordHash: hash,
		Timezone:     "UTC",
		IsAdmin:      true,
		Active:       true,
	}
	if err := st.CreateUser(u); err != nil {
		fmt.Fprintf(stderr, "times admin create: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "created admin %s with initial password: %s\n", u.Email, password)
	return 0
}

func adminUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: times admin create --email x --name y [--db path]")
	return 2
}
