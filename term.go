package main

import (
	"fmt"
	ui "github.com/cfstras/pcm/Godeps/_workspace/src/github.com/gizak/termui"
)

func selectConnection(conf *Configuration, input string) *Connection {
	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

	connectionsIndex := make(map[int]Node)

	treeView := NewSelectList()
	treePrint(&treeView.Items, connectionsIndex, conf)
	treeView.Height = ui.TermHeight() - 3

	searchView := ui.NewPar(input)
	searchView.Height = 3

	connectButton := ui.NewPar("Connect")
	connectButton.TextBgColor = ui.ColorBlue
	connectButton.Height = 3

	menuView := ui.NewRow(
		ui.NewCol(12, 0, connectButton))
	menuView.Height = 3

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, treeView)),
		ui.NewRow(
			ui.NewCol(6, 0, searchView),
			ui.NewCol(6, 0, menuView)))

	ui.Body.Align()

	events := ui.EventCh()
	for {
		ui.Render(ui.Body)
		ev := <-events
		if ev.Err != nil {
			fmt.Println(ev.Err)
		}
		switch ev.Type {
		case ui.EventKey:
			if ev.Key >= ui.KeyHome && ev.Key <= ui.KeyArrowRight {
				switch ev.Key {
				case ui.KeyHome:
					treeView.CurrentSelection = 0
				case ui.KeyEnd:
					treeView.CurrentSelection = len(treeView.Items) - 1
				case ui.KeyPgup:
					treeView.CurrentSelection -= treeView.Height - 3
				case ui.KeyPgdn:
					treeView.CurrentSelection += treeView.Height + 3
				case ui.KeyArrowDown:
					treeView.CurrentSelection++
				case ui.KeyArrowUp:
					treeView.CurrentSelection--
				}
			} else if ev.Key == ui.KeyEnter {
				n := connectionsIndex[treeView.CurrentSelection]
				if c, ok := n.(*Connection); ok {
					return c
				} else if _, ok := n.(*Container); ok {
					//TODO expand/unexpand
				}
			} else if ev.Key == ui.KeyEsc || ev.Key == ui.KeyCtrlC {
				return nil
			} else if ev.Ch >= ' ' && ev.Ch <= '~' {
				input += string(ev.Ch)
				searchView.Text = input
				//TODO re-filter
			} else if ev.Key == ui.KeyBackspace || ev.Key == ui.KeyBackspace2 {
				if len(input) > 0 {
					input = input[:len(input)-1]
					searchView.Text = input
					//TODO re-filter
				}
			}
		}
	}
}

type SelectList struct {
	ui.Block

	upperList, lowerList *ui.List
	middle               *ui.Par

	Items            []string
	CurrentSelection int
}

func NewSelectList() *SelectList {
	s := &SelectList{}
	s.upperList = ui.NewList()
	s.lowerList = ui.NewList()
	s.upperList.HasBorder = false
	s.lowerList.HasBorder = false

	s.HasBorder = true

	s.middle = ui.NewPar("")
	s.middle.Height = 1
	s.middle.HasBorder = false
	s.middle.BgColor = ui.ColorBlue

	s.upperList.Overflow = "wrap"
	s.lowerList.Overflow = "wrap"

	return s
}

func (s *SelectList) Buffer() []ui.Point {
	s.Align()

	ps := s.Block.Buffer()

	s.upperList.Items = s.Items[:s.CurrentSelection]
	s.middle.Text = s.Items[s.CurrentSelection]
	s.lowerList.Items = s.Items[s.CurrentSelection+1:]

	ps = append(ps, s.upperList.Buffer()...)
	ps = append(ps, s.middle.Buffer()...)
	ps = append(ps, s.lowerList.Buffer()...)

	return ps
}

func (s *SelectList) Align() {
	if s.CurrentSelection == 0 {
		s.upperList.IsDisplay = false
		s.lowerList.IsDisplay = true
	} else if s.CurrentSelection >= len(s.Items)-1 {
		s.upperList.IsDisplay = true
		s.lowerList.IsDisplay = false
	} else {
		s.upperList.IsDisplay = true
		s.lowerList.IsDisplay = true
	}

	s.upperList.Height = s.CurrentSelection
	s.lowerList.Height = s.Height - s.CurrentSelection - 1
	s.upperList.Width = s.Width
	s.lowerList.Width = s.Width

	s.lowerList.Y = s.CurrentSelection + 1
	s.middle.Y = s.CurrentSelection

	s.upperList.Align()
	s.lowerList.Align()
	s.middle.Align()
}
