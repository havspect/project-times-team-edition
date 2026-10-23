package cli

import (
	"fmt"
)

func (a *App) tokens(args []string) int {
	if len(args) == 0 {
		return a.usageLine("times tokens <create NAME|ls|rm ID>")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "create":
		return a.tokensCreate(rest)
	case "ls":
		return a.tokensList(rest)
	case "rm":
		return a.tokensRemove(rest)
	}
	return a.usageLine("times tokens <create NAME|ls|rm ID>")
}

func (a *App) tokensCreate(args []string) int {
	fs := newFlags("tokens create")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("tokens create", err)
	}
	if fs.NArg() != 1 {
		return a.usageLine("times tokens create <name>")
	}
	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("tokens create", err)
	}
	t, err := c.CreateToken(fs.Arg(0))
	if err != nil {
		return a.fail("tokens create", err)
	}
	fmt.Fprintf(a.Out, "created token %d %q\n%s\n", t.ID, t.Name, t.Token)
	fmt.Fprintln(a.Out, "The token is shown once; store it somewhere safe.")
	return 0
}

func (a *App) tokensList(args []string) int {
	fs := newFlags("tokens ls")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("tokens ls", err)
	}
	if code, ok := a.checkNoArgs("tokens ls", fs); !ok {
		return code
	}
	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("tokens ls", err)
	}
	tokens, err := c.ListTokens()
	if err != nil {
		return a.fail("tokens ls", err)
	}

	w := newTable(a.Out)
	fmt.Fprintln(w, "ID\tNAME\tCREATED\tLAST USED\tSTATUS")
	for _, t := range tokens {
		lastUsed, status := "-", "active"
		if t.LastUsedAt != nil {
			lastUsed = t.LastUsedAt.Local().Format(dateLayout)
		}
		if t.RevokedAt != nil {
			status = "revoked"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", t.ID, t.Name, t.CreatedAt.Local().Format(dateLayout), lastUsed, status)
	}
	_ = w.Flush()
	return 0
}

func (a *App) tokensRemove(args []string) int {
	fs := newFlags("tokens rm")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("tokens rm", err)
	}
	if fs.NArg() != 1 {
		return a.usageLine("times tokens rm <id>")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return a.fail("tokens rm", err)
	}
	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("tokens rm", err)
	}
	if err := c.RevokeToken(id); err != nil {
		return a.fail("tokens rm", err)
	}
	fmt.Fprintf(a.Out, "revoked token %d\n", id)
	return 0
}
