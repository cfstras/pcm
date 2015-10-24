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
	//treeView.Items = []string{"1 one", "2 two", "3 three", "4 four", "5 five",
	//	"6 six", "7 seven", "8 eight"}

	debugView := ui.NewPar("")
	debugView.Height = 5

	treeView.Height = ui.TermHeight() - 3 - debugView.Height

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
			ui.NewCol(12, 0, debugView)),
		ui.NewRow(
			ui.NewCol(6, 0, searchView),
			ui.NewCol(6, 0, menuView)))

	ui.Body.Align()

	events := ui.EventCh()
	for {
		ui.Render(ui.Body)
		ev := <-events
		if ev.Err != nil {
			debugView.Text = ev.Err.Error()
		}
		switch ev.Type {
		case ui.EventKey:
			if ev.Key <= ui.KeyHome && ev.Key >= ui.KeyArrowRight {
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
				if treeView.CurrentSelection > len(treeView.Items)-1 {
					treeView.CurrentSelection = len(treeView.Items) - 1
				} else if treeView.CurrentSelection < 0 {
					treeView.CurrentSelection = 0
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

		treeView.Align()
		if ev.Err == nil {
			debugView.Text = fmt.Sprintf(
				"ev: %d key: %x\ncur: %d scroll: %d scrolledCur: %d\ninner: %d",
				ev.Type, ev.Key, treeView.CurrentSelection, treeView.scroll,
				treeView.scrolledSelection, treeView.InnerHeight())
		}
	}
}

type SelectList struct {
	ui.Block

	upperList, lowerList ui.List
	middle               ui.Par

	Items            []string
	CurrentSelection int

	scroll            int
	scrolledSelection int
}

func NewSelectList() *SelectList {
	s := &SelectList{Block: *ui.NewBlock()}
	s.upperList = *ui.NewList()
	s.lowerList = *ui.NewList()
	s.upperList.HasBorder = false
	s.lowerList.HasBorder = false

	s.HasBorder = true

	s.middle = *ui.NewPar("")
	s.middle.Height = 1
	s.middle.HasBorder = false
	s.middle.TextBgColor = ui.ColorBlue

	s.upperList.Overflow = "wrap"
	s.lowerList.Overflow = "wrap"

	return s
}

func (s *SelectList) Buffer() []ui.Point {
	s.Align()

	ps := s.Block.Buffer()

	s.upperList.Items = s.Items[s.scroll:]
	s.middle.Text = s.Items[s.CurrentSelection]
	s.lowerList.Items = s.Items[s.CurrentSelection+1:]

	ps = append(ps, s.upperList.Buffer()...)
	ps = append(ps, s.middle.Buffer()...)
	ps = append(ps, s.lowerList.Buffer()...)

	return ps
}

func (s *SelectList) Align() {
	s.Block.Align()

	s.scrolledSelection = s.CurrentSelection - s.scroll

	if s.scrolledSelection >= s.InnerHeight() {
		s.scroll += s.InnerHeight() - s.scrolledSelection + 1

		s.scrolledSelection = s.CurrentSelection - s.scroll
	} else if s.scrolledSelection < 0 {
		s.scroll += s.scrolledSelection

		s.scrolledSelection = s.CurrentSelection - s.scroll
	}

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

	s.middle.Width = s.InnerWidth()
	s.upperList.Height = s.scrolledSelection
	s.lowerList.Height = s.InnerHeight() - s.scrolledSelection - 1
	s.upperList.Width = s.InnerWidth()
	s.lowerList.Width = s.InnerWidth()

	x := s.X + s.PaddingLeft
	y := s.Y + s.PaddingTop
	if s.HasBorder {
		x += 1
		y += 1
	}

	s.lowerList.X = x
	s.upperList.X = x
	s.middle.X = x

	s.upperList.Y = y
	s.lowerList.Y = y + s.scrolledSelection + 1
	s.middle.Y = y + s.scrolledSelection

	s.upperList.Align()
	s.lowerList.Align()
	s.middle.Align()
}
