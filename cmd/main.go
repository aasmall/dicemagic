package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	log "github.com/aasmall/dicemagic/lib/logger"

	"github.com/aasmall/dicemagic/lib/dicelang"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/jroimartin/gocui"
)

var client dicelang.RollerClient

func main() {
	log := log.New("dicemagic-cli", log.WithDebug(true), log.WithDefaultSeverity(logging.Info))

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
	diceMagicGRPCClient, err := grpc.DialContext(timeoutCtx, os.Args[1], grpcOpts...)
	if err != nil {
		log.Fatalf("did not connect to dice-server: %v", err)
	}
	client = dicelang.NewRollerClient(diceMagicGRPCClient)
	defer diceMagicGRPCClient.Close()

	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		log.Fatalf("failed to create UI: %v", err)
	}

	defer g.Close()
	g.Cursor = true
	g.SetManagerFunc(layout)

	if err := keybindings(g); err != nil {
		log.Fatalf("Could not set keybindings: %v", err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Fatalf("Error in Main Loop: %v", err)
	}

}

func cmdEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyDelete:
		v.EditDelete(false)
	case key == gocui.KeyEnter:
		return
	}
}

// Roll calls supplied grpc client with a freeform text command and returns a dice roll
func Roll(cmd string) (*dicelang.RollResponse, error) {
	timeOutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request := &dicelang.RollRequest{
		Cmd: cmd,
	}
	return client.Roll(timeOutCtx, request)
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() == "side" {
		_, err := g.SetCurrentView("main")
		return err
	}
	_, err := g.SetCurrentView("side")
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
	var l string
	var err error

	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		l = ""
	}

	maxX, maxY := g.Size()
	if v, err := g.SetView("msg", maxX/2-30, maxY/2, maxX/2+30, maxY/2+2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintln(v, l)
		if _, err := g.SetCurrentView("msg"); err != nil {
			return err
		}
	}
	return nil
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
	if err := g.SetKeybinding("side", gocui.KeyCtrlSpace, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyCtrlSpace, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyEnter, gocui.ModNone, getLine); err != nil {
		return err
	}
	if err := g.SetKeybinding("msg", gocui.KeyEnter, gocui.ModNone, delMsg); err != nil {
		return err
	}

	if err := g.SetKeybinding("main", gocui.KeyCtrlS, gocui.ModNone, saveMain); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyCtrlW, gocui.ModNone, saveVisualMain); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyCtrlW, gocui.ModNone, changeColor); err != nil {
		return err
	}
	if err := g.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, executeCommand); err != nil {
		return err
	}
	return nil
}
func executeCommand(g *gocui.Gui, v *gocui.View) error {
	cmd := v.Buffer()
	f, _ := g.View("main")
	result, _ := Roll(cmd)
	fmt.Fprintf(f, "%s\n----------\n", result)
	inputView, _ := g.View("input")
	inputView.SetCursor(0, 0)
	inputView.Clear()
	historyView, _ := g.View("side")
	fmt.Fprint(historyView, cmd)
	return nil
}
func changeColor(g *gocui.Gui, v *gocui.View) error {
	f, _ := g.View("footer")
	f.BgColor = f.BgColor + 1
	f.Clear()
	fmt.Fprint(f, f.BgColor)
	return nil
}
func saveMain(g *gocui.Gui, v *gocui.View) error {
	f, err := ioutil.TempFile("", "gocui_demo_")
	if err != nil {
		return err
	}
	defer f.Close()

	p := make([]byte, 5)
	v.Rewind()
	for {
		n, err := v.Read(p)
		if n > 0 {
			if _, err := f.Write(p[:n]); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func saveVisualMain(g *gocui.Gui, v *gocui.View) error {
	f, err := ioutil.TempFile("", "gocui_demo_")
	if err != nil {
		return err
	}
	defer f.Close()

	vb := v.ViewBuffer()
	if _, err := io.Copy(f, strings.NewReader(vb)); err != nil {
		return err
	}
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("side", 0, 0, 15, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "history"
		v.SelBgColor = gocui.ColorCyan
		v.SelFgColor = gocui.ColorBlack
	}
	if v, err := g.SetView("main", 16, 0, maxX-1, maxY-5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "rolls"
		v.Autoscroll = true
		v.Editable = false
		v.Wrap = true
	}
	if v, err := g.SetView("input", 16, maxY-4, maxX-1, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editor = gocui.EditorFunc(cmdEditor)
		v.SelBgColor = gocui.ColorCyan
		v.SelFgColor = gocui.ColorBlack
		v.Highlight = false
		v.Frame = true
		v.Title = "input"
		fmt.Fprint(v, ">>")
		v.Editable = true
		v.Wrap = false
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
		fmt.Fprintf(v, "dicemagic @ %s", os.Args[1])
	}
	return nil
}
