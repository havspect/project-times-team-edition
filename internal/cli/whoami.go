package cli

import "fmt"

func (a *App) whoami(args []string) int {
	fs := newFlags("whoami")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("whoami", err)
	}
	if code, ok := a.checkNoArgs("whoami", fs); !ok {
		return code
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("whoami", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("whoami", err)
	}

	fmt.Fprintf(a.Out, "%s <%s>\n", me.Name, me.Email)
	if me.IsAdmin {
		fmt.Fprintln(a.Out, "admin")
	}
	for _, m := range me.Memberships {
		fmt.Fprintf(a.Out, "%s of %s\n", m.Role, m.Team)
	}
	return 0
}
