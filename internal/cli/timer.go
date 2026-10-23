package cli

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/havspect/tstamps/internal/client"
)

func (a *App) start(args []string) int {
	fs := newFlags("start")
	urlFlag, tokenFlag := clientFlags(fs)
	note := fs.String("note", "", "note for the running entry")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("start", err)
	}
	if fs.NArg() != 1 {
		return a.usageLine("tstamps start <project> [--note \"...\"]")
	}
	ref := fs.Arg(0)

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("start", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("start", err)
	}
	project, err := a.resolveProject(c, ref)
	if err != nil {
		return a.fail("start", err)
	}

	e, err := c.StartTimer(project.ID, a.zone(me), *note)
	if err != nil {
		if running, ok := alreadyRunning(err); ok && running != nil {
			loc := entryLocation(*running, location(me))
			fmt.Fprintf(a.Err, "tstamps start: already running since %s on %s\n",
				formatClock(running.StartAt, loc), projectName(a.projectNames(c, me), running.ProjectID))
			return 1
		}
		return a.fail("start", err)
	}

	loc := entryLocation(*e, a.local())
	fmt.Fprintf(a.Out, "started %s on %s\n", formatClock(e.StartAt, loc), project.Name)
	return 0
}

func (a *App) stop(args []string) int {
	fs := newFlags("stop")
	urlFlag, tokenFlag := clientFlags(fs)
	note := fs.String("note", "", "note to append")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("stop", err)
	}
	if code, ok := a.checkNoArgs("stop", fs); !ok {
		return code
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("stop", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("stop", err)
	}
	e, err := c.StopTimer(*note)
	if err != nil {
		var apiErr *client.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			fmt.Fprintln(a.Err, "tstamps stop: no running entry")
			return 1
		}
		return a.fail("stop", err)
	}

	loc := entryLocation(*e, a.local())
	end := e.StartAt
	if e.EndAt != nil {
		end = *e.EndAt
	}
	fmt.Fprintf(a.Out, "stopped %s (%s) on %s\n",
		formatClock(end, loc), formatSeconds(secondsOf(*e, a.Now())), projectName(a.projectNames(c, me), e.ProjectID))
	return 0
}
