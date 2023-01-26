package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/urfave/cli/v2"

	"github.com/capatazlib/go-capataz/saboteur/api"
)

const (
	fgDefault string = "\033[0;0m"
	fgRed     string = "\033[1;31m"
)

// UI represents an interactive UI
type UI struct {
	plans           []api.Plan
	nodes           []api.Node
	g               *gocui.Gui
	refreshInterval time.Duration
	refresh         chan (interface{})
}

func (ui *UI) loop(ctx context.Context) {
	t := time.NewTicker(ui.refreshInterval)
	defer t.Stop()
	ui.g.Update(ui.fetchAndUpdate)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ui.refresh:
			ui.g.Update(ui.fetchAndUpdate)
		case <-t.C:
			ui.g.Update(ui.fetchAndUpdate)
		}
	}
}

func (ui *UI) fetch() error {
	nodes, err := listNodes()
	if err != nil {
		return err
	}
	ui.nodes = nodes.Nodes

	plans, err := listPlans()
	if err != nil {
		return err
	}
	ui.plans = plans.Plans
	return nil
}

func interactive(c *cli.Context) error {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return errorf("something went wrong: %s", err)
	}
	defer g.Close()
	ui := &UI{
		g:               g,
		refreshInterval: c.Duration("refresh"),
		refresh:         make(chan interface{}, 1),
	}

	go ui.loop(c.Context)

	g.Cursor = true
	g.SetManagerFunc(ui.layout)

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	if err := g.SetKeybinding("", 'q', gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	lnUpAct := func(cg *gocui.Gui, v *gocui.View) error {
		v.MoveCursor(0, -1, false)
		return nil
	}
	lnDownAct := func(cg *gocui.Gui, v *gocui.View) error {
		v.MoveCursor(0, 1, false)
		return nil
	}
	if err := g.SetKeybinding("plans", gocui.KeyArrowUp, gocui.ModNone, lnUpAct); err != nil {
		return err
	}
	if err := g.SetKeybinding("plans", gocui.KeyArrowDown, gocui.ModNone, lnDownAct); err != nil {
		return err
	}
	if err := g.SetKeybinding("plans", 'k', gocui.ModNone, lnUpAct); err != nil {
		return err
	}
	if err := g.SetKeybinding("plans", 'j', gocui.ModNone, lnDownAct); err != nil {
		return err
	}
	if err := g.SetKeybinding("plans", gocui.KeySpace, gocui.ModNone, func(cg *gocui.Gui, v *gocui.View) error {
		_, y := v.Cursor()
		_, yy := v.Origin()
		n := y + yy
		if n >= len(ui.plans) {
			// Cursor is below bottom of the list
			return nil
		}
		plan := ui.plans[n]
		if plan.Running {
			err := stop(plan.Name)
			if err != nil {
				return err
			}
		} else {
			err := start(plan.Name)
			if err != nil {
				return err
			}
		}
		ui.refresh <- struct{}{}
		return nil
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("plans", 'r', gocui.ModNone, func(cg *gocui.Gui, v *gocui.View) error {
		ui.refresh <- struct{}{}
		return nil
	}); err != nil {
		return err
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	return nil
}

func (ui *UI) layout(g *gocui.Gui) error {
	// Draw three boxes:
	// +-----------------------+
	// |           top         |
	// +---+-------------------|
	// | n |                   |
	// | o |                   |
	// | d |      plans        |
	// | e |                   |
	// | s |                   |
	// +---+-------------------+
	maxX, maxY := g.Size()

	if _, err := g.SetView("top", -1, -1, maxX, 3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
	}
	if v, err := g.SetView("nodes", -1, 3, 50, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "nodes"
	}
	if v, err := g.SetView("plans", 50, 3, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "plans"
	}
	if _, err := g.SetCurrentView("plans"); err != nil {
		return err
	}
	err := ui.update(g)
	return err
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (ui *UI) fetchAndUpdate(g *gocui.Gui) error {
	err := ui.fetch()
	if err != nil {
		return err
	}
	return ui.update(g)
}

func (ui *UI) update(g *gocui.Gui) error {
	v, _ := g.View("top")
	v.Clear()
	fmt.Fprintln(v, `"q" or CTRL-C to quit`)
	fmt.Fprintf(v, "\"r\" to refresh (auto refresh every %s)\n", ui.refreshInterval.String())
	fmt.Fprintln(v, `SPACE to start/stop a plan`)

	v, _ = g.View("plans")
	v.Clear()
	if ui.plans == nil {
		fmt.Fprintln(v, "Fetching...")
	} else {
		for _, p := range ui.plans {
			running := ""
			if p.Running {
				running = red("Running")
			}
			fmt.Fprintf(
				v,
				"%s\t%s\n",
				p.Name,
				running,
			)
		}
	}

	v, _ = g.View("nodes")
	v.Clear()
	if ui.nodes != nil {
		for _, n := range ui.nodes {
			fmt.Fprintln(v, n.Name)
		}
	}
	return nil
}

func red(s string) string {
	return fmt.Sprintf("%s%s%s", fgRed, s, fgDefault)
}
