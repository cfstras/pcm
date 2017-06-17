package main

import (
	"fmt"
	"strings"

	"github.com/cfstras/go-utils/math"
	"github.com/cfstras/pcm/types"

	ui "github.com/gizak/termui"
	"github.com/renstrom/fuzzysearch/fuzzy"
)

func selectConnection(conf *types.Configuration, input string) *types.Connection {
	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer func() {
		ui.Close()
		if e := recover(); e != nil {
			panic(e)
		}
	}()
	treeView := NewSelectList()
	treeView.Border.Label = " Connections "

	debugView := ui.NewPar("")

	searchView := ui.NewPar(input)
	searchView.Border.Label = " Search "

	connectButton := ui.NewPar(" Connect ")

	menuView := ui.NewRow(
		ui.NewCol(8, 0, connectButton),
	)

	selectedButton := 0
	buttons := []*ui.Par{connectButton}

	selectButtons := func() {
		selectedButton %= len(buttons)
		for i, v := range buttons {
			if i == selectedButton {
				v.TextBgColor = ui.ColorBlue
			} else {
				v.TextBgColor = ui.ColorDefault
			}
		}
	}
	selectButtons()

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

	connectionsIndex := make(map[int]types.Node)
	pathToIndexMap := make(map[string]int)
	var distances map[string]int
	var filteredRoot *types.Container

	doRefilter := func() {
		pathToIndexMap := make(map[string]int)
		distances, _ = filter(conf, input)
		filteredRoot = filterTree(conf, distances)
		drawTree(treeView, connectionsIndex, distances, pathToIndexMap, filteredRoot)
		/*if bestMatchPath != "" {
			if index, ok := pathToIndexMap[bestMatchPath]; ok {
				treeView.CurrentSelection = index
			}
		}*/
	}
	doRefilter()

	events := make(chan ui.Event)

	go func(in <-chan ui.Event, out chan<- ui.Event) {
		for e := range in {
			out <- e
		}
		close(out)
	}(ui.EventCh(), events)
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
				case ui.KeyArrowLeft:
					selectedButton--
					selectButtons()
				case ui.KeyArrowRight:
					selectedButton++
					selectButtons()
				}
				if treeView.CurrentSelection > len(treeView.Items)-1 {
					treeView.CurrentSelection = len(treeView.Items) - 1
				} else if treeView.CurrentSelection < 0 {
					treeView.CurrentSelection = 0
				}
			} else if ev.Key == ui.KeyEnter {
				n := connectionsIndex[treeView.CurrentSelection]
				if c, ok := n.(*types.Connection); ok {
					if buttons[selectedButton] == connectButton {
						return c
					}
				} else if c, ok := n.(*types.Container); ok {
					if c.Expanded {
						c.Expanded = false
					} else {
						c.Expanded = true
					}
					drawTree(treeView, connectionsIndex, distances, pathToIndexMap, filteredRoot)
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
			doRefilter()
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
				if c, ok := n.(*types.Connection); ok {
					debugView.Text = fmt.Sprintf("%s %s:%d\n%s",
						c.Info.Protocol, c.Info.Host, c.Info.Port, c.Info.Description)
				} else if _, ok := n.(*types.Container); ok {
					debugView.Text = ""
				}
			}
		}
	}
}

func filterTree(conf *types.Configuration, distances map[string]int) *types.Container {
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

func filterTreeDescend(pathPrefix string, node *types.Container, distances map[string]int) {
	newContainers := []types.Container{}
	for _, c := range node.Containers { // this implicitly copies the struct
		nextPathPrefix := pathPrefix + c.Name + "/"
		if k, _ := pathPrefixInDistances(nextPathPrefix, distances); k {
			newContainers = append(newContainers, c)
			nc := &newContainers[len(newContainers)-1]
			nc.Expanded = true
			filterTreeDescend(nextPathPrefix, nc, distances)
		}
	}
	minDist := math.MaxInt
	maxDist := 0
	for _, d := range distances {
		maxDist = math.MaxI(maxDist, d)
		minDist = math.MinI(minDist, d)
	}

	newConnections := []types.Connection{}
	for _, c := range node.Connections {
		if k, d := pathPrefixInDistances(pathPrefix+c.Name, distances); k {
			df := 1 - (float32(d)-float32(minDist))/(float32(maxDist)-float32(minDist))
			df *= df
			scores := make([]float32, 3)
			scores[0] = math.MinF(1/3.0, df)
			df -= 1 / 3.0
			scores[1] = math.MinF(1/3.0, df)
			df -= 1 / 3.0
			scores[2] = math.MinF(1/3.0, df)
			c.StatusInfo = Sparkline(scores, 0, 1/3.0)

			newConnections = append(newConnections, c)
		}
	}

	node.Containers = newContainers
	node.Connections = newConnections
}

func pathPrefixInDistances(nextPathPrefix string, distances map[string]int) (bool, int) {
	for k, v := range distances {
		if strings.HasPrefix(k, nextPathPrefix) {
			return true, v
		}
	}
	return false, -1
}

func drawTree(treeView *SelectList, connectionsIndex map[int]types.Node,
	distances map[string]int, pathToIndexMap map[string]int, node *types.Container) {
	items := make([]string, 0, len(treeView.Items))
	treePrint(&items, connectionsIndex, pathToIndexMap, node, treeView.InnerWidth())

	if len(items) == 0 {
		treeView.Items = []string{"   No Results for search... ☹  "}
	} else {
		treeView.Items = items
	}
}

func filter(conf *types.Configuration, input string) (map[string]int, string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ""
	}
	connections := types.ListConnections(conf, true)
	words := listWords(connections)
	suggs := fuzzy.RankFindFold(input, words)
	res := make(map[string]int)

	minDist := math.MaxInt
	minPath := ""
	for _, s := range suggs {
		if s.Distance < minDist {
			minDist = s.Distance
			minPath = s.Target
		}
		res[s.Target] = s.Distance
	}
	return res, minPath
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
