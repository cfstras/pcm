package main

import (
	"fmt"
	"strings"

	"github.com/cfstras/go-utils/math"
	ui "github.com/cfstras/pcm/Godeps/_workspace/src/github.com/gizak/termui"
	"github.com/cfstras/pcm/Godeps/_workspace/src/github.com/renstrom/fuzzysearch/fuzzy"
)

func selectConnection(conf *Configuration, input string) *Connection {
	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()
	treeView := NewSelectList()
	treeView.Border.Label = " Connections "

	debugView := ui.NewPar("")

	searchView := ui.NewPar(input)
	searchView.Border.Label = " Search "

	connectButton := ui.NewPar(" Connect ")
	connectButton.TextBgColor = ui.ColorBlue

	menuView := ui.NewRow(
		ui.NewCol(12, 0, connectButton))

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, treeView)),
		ui.NewRow(
			ui.NewCol(12, 0, debugView)),
		ui.NewRow(
			ui.NewCol(6, 0, searchView),
			ui.NewCol(6, 0, menuView)))

	heights := func() {
		searchView.Height = 3
		connectButton.Height = searchView.Height
		menuView.Height = searchView.Height
		debugView.Height = 5
		treeView.Height = ui.TermHeight() - searchView.Height - debugView.Height
	}
	heights()

	ui.Body.Align()

	connectionsIndex := make(map[int]Node)
	distances := filter(conf, input)
	filteredRoot := filterTree(conf, distances)

	drawTree(treeView, connectionsIndex, distances, filteredRoot)

	events := ui.EventCh()
	for {
		ui.Render(ui.Body)
		ev := <-events
		if ev.Err != nil {
			debugView.Text = ev.Err.Error()
		}

		refilter := false
		switch ev.Type {
		case ui.EventResize:
			heights()
			ui.Body.Width = ev.Width
			ui.Body.Align()
			treeView.Debug += "  resize"
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
				} else if c, ok := n.(*Container); ok {
					if c.Expanded {
						c.Expanded = false
					} else {
						c.Expanded = true
					}
					drawTree(treeView, connectionsIndex, distances, filteredRoot)
				}

			} else if ev.Key == ui.KeyEsc || ev.Key == ui.KeyCtrlC {
				return nil
			} else if ev.Ch >= ' ' && ev.Ch <= '~' {
				input += string(ev.Ch)
				searchView.Text = input

				refilter = true
			} else if ev.Key == ui.KeyBackspace || ev.Key == ui.KeyBackspace2 {
				if len(input) > 0 {
					input = input[:len(input)-1]
					searchView.Text = input
					refilter = true
				}
			}
		}

		if refilter {
			distances = filter(conf, input)
			filteredRoot = filterTree(conf, distances)
			drawTree(treeView, connectionsIndex, distances, filteredRoot)
		}

		if ev.Err == nil {
			if DEBUG {
				debugView.Text = fmt.Sprint(distances)
				debugView.Text += fmt.Sprintf(
					" ev: %d key: %x input: %s|\ncur: %d scroll: %d scrolledCur: %d len: %d\ninner: %d align: %s",
					ev.Type, ev.Key, input, treeView.CurrentSelection, treeView.scroll,
					treeView.scrolledSelection, len(treeView.Items), treeView.InnerHeight(), treeView.Debug)
				treeView.Debug = ""
			} else {
				n := connectionsIndex[treeView.CurrentSelection]
				if c, ok := n.(*Connection); ok {
					debugView.Text = fmt.Sprintf("%s %s:%d\n%s",
						c.Info.Protocol, c.Info.Host, c.Info.Port, c.Info.Description)
				} else if _, ok := n.(*Container); ok {
					debugView.Text = ""
				}
			}
		}
	}
}

func filterTree(conf *Configuration, distances map[string]int) *Container {
	if distances == nil {
		return &conf.Root
	}
	if len(distances) == 0 {
		return nil
	}
	filteredRoot := conf.Root
	filterTreeDescend("/", &filteredRoot, distances)

	return &filteredRoot
}

func filterTreeDescend(pathPrefix string, node *Container, distances map[string]int) {
	newContainers := []Container{}
	for _, c := range node.Containers { // this implicitly copies the struct
		nextPathPrefix := pathPrefix + c.Name + "/"
		if pathPrefixInDistances(nextPathPrefix, distances) {
			newContainers = append(newContainers, c)
			nc := &newContainers[len(newContainers)-1]
			nc.Expanded = true
			filterTreeDescend(nextPathPrefix, nc,
				distances)
		}
	}
	newConnections := []Connection{}
	for _, c := range node.Connections {
		if pathPrefixInDistances(pathPrefix+c.Name, distances) {
			newConnections = append(newConnections, c)
		}
	}

	node.Containers = newContainers
	node.Connections = newConnections
}

func pathPrefixInDistances(nextPathPrefix string, distances map[string]int) bool {
	for k := range distances {
		if strings.HasPrefix(k, nextPathPrefix) {
			return true
		}
	}
	return false
}

func drawTree(treeView *SelectList, connectionsIndex map[int]Node,
	distances map[string]int, node *Container) {
	treeView.Items = treeView.Items[:0]
	treePrint(&treeView.Items, connectionsIndex, node)

	if len(treeView.Items) == 0 {
		treeView.Items = []string{"   No Results for search... ☹  "}
	}
}

func filter(conf *Configuration, input string) map[string]int {
	input = strings.Trim(input, " \n\r\t")
	if input == "" {
		return nil
	}
	connections := listConnections(conf, true)
	words := listWords(connections)
	suggs := fuzzy.RankFindFold(input, words)
	res := make(map[string]int)
	for _, s := range suggs {
		res[s.Target] = s.Distance
	}
	return res
}

type SelectList struct {
	ui.Block

	upperList, lowerList ui.List
	middle               ui.Par

	Items            []string
	CurrentSelection int

	scroll            int
	scrolledSelection int

	Debug string
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

	if s.CurrentSelection >= len(s.Items) {
		s.CurrentSelection = len(s.Items) - 1
	}

	inner := s.InnerHeight() - 1
	s.scrolledSelection = s.CurrentSelection - s.scroll
	if DEBUG {
		s.Debug += fmt.Sprintf("scrolled: %d height: %d  ", s.scrolledSelection, s.InnerHeight())
	}

	if s.scrolledSelection >= inner {
		if DEBUG {
			s.Debug += fmt.Sprintf("adjusting scroll %d  ", s.scrolledSelection-inner)
		}
		s.scroll += s.scrolledSelection - inner
	} else if s.scrolledSelection < 0 {
		if DEBUG {
			s.Debug += fmt.Sprintf("adjusting scroll - %d  ", math.AbsI(s.scrolledSelection))
		}
		s.scroll -= math.AbsI(s.scrolledSelection)
	}
	if DEBUG {
		s.Debug += fmt.Sprintf("scrolled: %d  ", s.scrolledSelection)
	}
	s.scrolledSelection = s.CurrentSelection - s.scroll

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
