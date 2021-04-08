package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"image"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aasmall/asciigraph"
	log "github.com/aasmall/dicemagic/lib/logger"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/viper"

	"github.com/aasmall/dicemagic/lib/dicelang"
	errors "github.com/aasmall/dicemagic/lib/dicelang-errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/aasmall/gocui"
	"github.com/mgutz/ansi"
)

const (
	fullBlock   = '█'
	bottomBlock = '▄'
	topBlock    = '▀'
)

var (
	version string
	appname = "dicemagic"
)

var (
	client    dicelang.RollerClient
	grpcConn  *grpc.ClientConn
	ansiReset string
	hist      *history
)

// configs
var (
	// set defaults used before config file exists
	defaultConfigs = map[string]string{
		"endpoint":     "grpc.dicemagic.io:443",
		"prompt-color": "8",
		"crit-color":   "100",
		"history-file": ".history",
		"cmd-file":     ".cmds",
	}
)

type historyItem struct {
	cmd      string
	response *dicelang.RollResponse
}
type history struct {
	items []historyItem
}

func init() {
	for k, v := range defaultConfigs {
		viper.SetDefault(k, v)
	}

	viper.SetConfigName("config")                          // name of config file (without extension)
	viper.SetConfigType("yaml")                            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")                               // optionally look for config in the working directory
	viper.AddConfigPath(fmt.Sprintf("/etc/%s/", appname))  // path to look for the config file in
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appname)) // call multiple times to add many search paths

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			viper.SafeWriteConfig()
		} else {
			panic(fmt.Errorf("fatal error config file: %s", err))
		}
	}

}

func main() {
	log := log.New("dicemagic-cli", log.WithDebug(true))
	ansiReset = ansi.ColorCode("reset")
	endpoint := viper.GetString("endpoint")
	err := connect(endpoint)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer grpcConn.Close()

	g, err := gocui.NewGui(gocui.Output256)

	if err != nil {
		log.Fatalf("failed to create UI: %v", err)
	}
	defer g.Close()

	// defer ui.Close()

	viper.WatchConfig()

	g.Cursor = true
	g.Mouse = true

	g.SetManagerFunc(layout)

	hist = &history{}

	// keep connected
	go func() {
		ticker := time.NewTicker(time.Millisecond * 50)
		defer ticker.Stop()
		for range ticker.C {
			g.Update(func(g *gocui.Gui) error {
				footerPane, _ := g.View("footer")
				footerPane.Clear()
				width, _ := footerPane.Size()
				fmt.Fprint(footerPane, rightAlign(fmt.Sprintf("dicemagic @ %s", viper.GetString("endpoint")), grpcConn.GetState().String(), width))
				return nil
			})
		}
	}()

	if err := keybindings(g); err != nil {
		log.Fatalf("Could not set keybindings: %v", err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Fatalf("Error in Main Loop: %v", err)
	}

}
func (h *history) add(items ...historyItem) {
	h.items = append(h.items, items...)
}
func rightAlign(leftString, rightString string, width int) string {
	b := strings.Builder{}
	b.WriteString(leftString)
	for i := 0; i < width-len(leftString)-len(rightString); i++ {
		b.WriteRune(' ')
	}
	b.WriteString(rightString)
	return b.String()
}
func connect(grpcServer string) error {
	certpool, _ := x509.SystemCertPool()
	transportCreds := credentials.NewTLS(&tls.Config{
		RootCAs: certpool,
	})
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	grpcOpts := []grpc.DialOption{
		grpc.FailOnNonTempDialError(true),
		grpc.WithTransportCredentials(transportCreds),
	}
	diceMagicGRPCClient, err := grpc.DialContext(timeoutCtx, grpcServer, grpcOpts...)
	if err != nil {
		return fmt.Errorf("did not connect to dice-server: %v", err)
	}
	client = dicelang.NewRollerClient(diceMagicGRPCClient)
	grpcConn = diceMagicGRPCClient
	return nil
}
func cmdEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyEnter:
		return
	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyDelete:
		v.EditDelete(false)
	case key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, true)
	case key == gocui.KeyArrowRight:
		v.MoveCursor(1, 0, true)
	}
}

// Roll calls supplied grpc client with a freeform text command and returns a dice roll
func Roll(cmd string) (*dicelang.RollResponse, error) {
	timeOutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request := &dicelang.RollRequest{
		Cmd:           cmd,
		Probabilities: true,
		Chart:         true,
	}
	return client.Roll(timeOutCtx, request)
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() == "side" {
		_, err := g.SetCurrentView("input")
		v, _ := g.View("side")
		g.Cursor = true
		v.Highlight = false
		return err
	}
	v, err := g.SetCurrentView("side")
	g.Cursor = false
	v.Highlight = true
	return err
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}

func getLine(g *gocui.Gui, v *gocui.View) error {
	// _, _ := g.Size()
	_, cy := v.Cursor()
	sets := hist.items[cy].response.DiceSets
	setLabels := []string{}
	streams := [][]float64{}
	probabilities := []map[int64]float64{}
	totals := []int64{}
	longestSet := 0
	for _, set := range sets {
		for _, die := range set.Dice {
			// probStringBuilder.WriteString(asciiChart(dicelang.DiceProbability(die.Count, die.Sides, set.DropHighest, set.DropLowest)))
			probs := dicelang.DiceProbability(die.Count, die.Sides, set.DropHighest, set.DropLowest)
			intKeys := []int64{}
			for key := range probs {
				intKeys = append(intKeys, key)
				setLabels = append(setLabels, strconv.FormatInt(key, 10))
			}
			sort.Slice(intKeys, func(i, j int) bool { return intKeys[i] < intKeys[j] })
			values := []float64{}
			for _, intKey := range intKeys {
				values = append(values, probs[intKey])
			}
			if longestSet < len(probs) {
				longestSet = len(probs)
			}
			streams = append(streams, values)
			probabilities = append(probabilities, probs)
			totals = append(totals, die.Total)
		}
	}
	chart := asciigraph.Plot(streams[0], asciigraph.Caption("caption string"), asciigraph.Height(23), asciigraph.Width(50))
	// probs := probStringBuilder.String()
	// used for widget

	mv, _ := g.View("main")
	fmt.Fprintf(mv, "map: %+v\n\n", probabilities)

	p1 := widgets.NewBarChart()
	p1.Data = streams[0]
	p1.Labels = setLabels
	p1.SetRect(0, 0, 50, 25)
	p1.BarGap = 0
	p1.BarWidth = 3
	p1.NumFormatter = func(f float64) string {
		s := strconv.FormatFloat(f, 'f', 4, 64)
		sb := strings.Builder{}
		for _, r := range s {
			sb.WriteRune(r)
			sb.WriteRune('\n')
		}
		sb.WriteString("%")
		return sb.String()
	}

	drawRectangle := image.Rectangle{image.Point{0, 0}, image.Point{50, 25}}
	xScale, yScale := scalePoints(drawRectangle, probabilities...)

	buf := ui.NewBuffer(drawRectangle)
	originX := buf.Dx()
	originY := buf.Dy()

	p2 := ui.NewCanvas()
	p2.SetRect(0, 0, 50, 25)
	for i, v := range probabilities {
		for x, y := range v {
			if x == totals[i] {
				p2.SetPoint(image.Point{originX - int(float64(x)*xScale), originY - int(y*yScale)/2}, ui.ColorRed)
			} else {
				p2.SetPoint(image.Point{originX - int(float64(x)*xScale), originY - int(y*yScale)/2}, ui.ColorBlue)
			}
		}
	}

	// for i, probs := range probabilities {
	// 	for roll, prob := range probs {
	// 		drawY := roundToUnit((prob*yScale)/2, 0.5)
	// 		var r rune
	// 		if int(drawY) != int(math.Ceil(drawY)) {
	// 			r = topBlock
	// 		} else {
	// 			r = bottomBlock
	// 		}
	// 		point := image.Point{originX - int(float64(roll)*xScale), originY - int(prob*yScale)/2}
	// 		style := ui.NewStyle(ui.ColorBlue)
	// 		if totals[i] == roll {
	// 			style.Fg = ui.ColorMagenta
	// 		}
	// 		if buf.CellMap[point].Rune == topBlock || buf.CellMap[point].Rune == bottomBlock {
	// 			buf.CellMap[point] = ui.Cell{Rune: fullBlock, Style: style}
	// 		} else {
	// 			buf.CellMap[point] = ui.Cell{Rune: r, Style: style}
	// 		}
	// 		//fmt.Fprintf(mv, "point: %+v\n", point)
	// 	}
	// }

	//fmt.Fprintf(mv, "cells: %+v", buf.CellMap)
	p1.Draw(buf)
	if v, err := g.SetView(rectToCoords("msg", drawRectangle.Dy(), drawRectangle.Dx()+3, g)); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Wrap = false
		v.Frame = true
		//v.WriteWidget(*buf)
		fmt.Fprint(v, chart)

		// fmt.Fprintf(v, "\nminX: %d, maxX: %d, minY: %f, maxY:%f", minX, maxX, minY, maxY)
		// fmt.Fprintf(v, "\nScaleX: %f, ScaleY: %f\n", xScale, yScale)
		if _, err := g.SetCurrentView("msg"); err != nil {
			return err
		}
	}
	return nil
}

func scalePoints(targetRect image.Rectangle, source ...map[int64]float64) (float64, float64) {
	var minX, maxX int64
	var minY, maxY, yScaleFactor, xScaleFactor float64
	minX = math.MaxInt64
	minY = math.MaxFloat64
	for _, pointMap := range source {
		for x, y := range pointMap {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX == minX {
		xScaleFactor = 0
	} else {
		xScaleFactor = float64(targetRect.Dx()) / float64(maxX)
	}
	if maxY == minY {
		yScaleFactor = 0
	} else {
		yScaleFactor = float64(targetRect.Dy()*2) / maxY
	}
	return xScaleFactor, yScaleFactor
}
func roundToUnit(x, unit float64) float64 {
	return math.Round(x/unit) * unit
}
func rectToCoords(name string, height, width int, g *gocui.Gui) (string, int, int, int, int) {
	maxX, maxY := g.Size()
	x0 := int(math.Ceil(float64(maxX)/2 - float64(width)/2))
	x1 := int(math.Ceil(float64(maxX)/2 + float64(width)/2))
	y0 := int(math.Ceil(float64(maxY)/2 - float64(height)/2))
	y1 := int(math.Ceil(float64(maxY)/2+float64(height)/2) + 1)
	return name, x0, y0, x1, y1
}
func delMsg(g *gocui.Gui, v *gocui.View) error {
	if err := g.DeleteView("msg"); err != nil {
		return err
	}
	if _, err := g.SetCurrentView("side"); err != nil {
		return err
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func keybindings(g *gocui.Gui) error {

	//static keybinds
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyEnter, gocui.ModNone, getLine); err != nil {
		return err
	}
	if err := g.SetKeybinding("msg", gocui.KeyEnter, gocui.ModNone, delMsg); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyCtrlW, gocui.ModNone, changeColor); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, executeCommand); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		return err
	}
	return nil
}
func executeCommand(g *gocui.Gui, v *gocui.View) error {
	promptColor := viper.GetString("prompt-color")
	cmd := strings.TrimSpace(v.Buffer())
	mainView, _ := g.View("main")
	result, err := Roll(cmd)
	item := historyItem{cmd: cmd, response: result}
	prompt := ansi.Color(">", promptColor)
	fmt.Fprintf(mainView, "%s%s\n", prompt, cmd)
	if err != nil {
		fmt.Fprintf(mainView, "grpc error: %s\n\n", err)
	} else if result.Ok {
		indented := result.StringFromRollResponse()
		var formatted strings.Builder
		for _, line := range strings.Split(indented, "\n") {
			formatted.WriteRune(' ')
			formatted.WriteString(line)
			formatted.WriteRune('\n')
		}
		fmt.Fprintf(mainView, "%s\n", formatted.String())
		hist.add(item)
	} else if result.Error != nil {
		switch result.Error.Code {
		case errors.InvalidAST:
			fallthrough
		case errors.Friendly:
			fmt.Fprintf(mainView, "%s\n\n", result.Error.Msg)
		case errors.Unexpected:
			fmt.Fprintf(mainView, "Unexpected Error: %s\n\n", result.Error.Msg)
		case errors.InvalidCommand:
			fmt.Fprintf(mainView, "Invalid Command: %s\n\n", result.Error.Msg)
		default:
			fmt.Fprintf(mainView, "Unhandled Error: %s\n\n", result.Error.String())
		}
	}
	inputView, _ := g.View("input")
	inputView.SetCursor(0, 0)
	inputView.Clear()
	historyView, _ := g.View("side")
	historyView.Clear()
	for _, i := range hist.items {
		fmt.Fprint(historyView, i.cmd+"\n")
	}
	return nil
}
func changeColor(g *gocui.Gui, v *gocui.View) error {
	f, _ := g.View("footer")
	f.BgColor = f.BgColor + 1
	f.Clear()
	fmt.Fprint(f, f.BgColor)
	return nil
}
func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("side", 0, 0, 19, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "history"
		v.SelBgColor = gocui.ColorCyan
		v.SelFgColor = gocui.ColorBlack
	}
	if v, err := g.SetView("main", 20, 0, maxX-1, maxY-5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "rolls"
		v.Autoscroll = true
		v.Editable = false
		v.Wrap = true
	}
	if v, err := g.SetView("input", 20, maxY-4, maxX-1, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// v.Editor = gocui.EditorFunc(cmdEditor)
		v.SelBgColor = gocui.ColorCyan
		v.SelFgColor = gocui.ColorBlack
		v.Highlight = false
		v.Frame = true
		v.Editable = true
		v.Wrap = false
		v.Title = "input"
		if _, err := g.SetCurrentView("input"); err != nil {
			return err
		}
	}
	if v, err := g.SetView("footer", -1, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.BgColor = 238
		v.FgColor = gocui.ColorWhite
		v.Highlight = false
		v.Frame = false
		fmt.Fprintf(v, "dicemagic @ %s : %s", viper.GetString("endpoint"), grpcConn.GetState().String())
	}
	return nil
}
func lineCounter(s string) (int, error) {
	r := strings.NewReader(s)
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}
func columnCounter(s string) int {
	lines := strings.Split(s, "\n")
	length := 0
	for _, line := range lines {
		if len(line) > length {
			length = len(line)
		}
	}
	return length
}

func asciiChart(probs map[int64]float64) string {
	chartBuilder := strings.Builder{}
	keys := make([]int64, 0, len(probs))
	for k := range probs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		chartBuilder.WriteString(strconv.FormatInt(k, 10))
		chartBuilder.WriteString(" : ")
		chartBuilder.WriteString(strconv.FormatFloat(probs[k], 'f', 8, 64))
		chartBuilder.WriteRune('\n')
	}
	return chartBuilder.String()
}
