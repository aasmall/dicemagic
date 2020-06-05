package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a mock slack message
type Message struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	User    string `json:"user"`
	Text    string `json:"text"`
	Ts      string `json:"ts"`
}

type arrayFlags []string

func (f *arrayFlags) String() string {
	var b bytes.Buffer
	for i, flag := range *f {
		b.WriteString(flag)
		if i+1 != len(*f) {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

func (f *arrayFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

var kubePortFwd arrayFlags

func main() {
	var addr = flag.String("addr", "localhost:8080", "http service address")
	var kubeSvc = flag.String("kube-svc", "mock-slack-server", "kubectl port-forward service name")
	flag.Var(&kubePortFwd, "kube-port-fwd", "kubectl port-forward rules. E.g. --kubePortFwd 8443:1082 --kubePortFwd 8080:2082")
	flag.Parse()

	log.SetFlags(0)
	if kubePortFwd.String() == "" {
		kubePortFwd.Set("8080:2082")
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	portFwdProcess := startKubectlPortForward(kubeSvc, kubePortFwd)
	defer func() {
		portFwdProcess.Kill()
	}()

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/wss/snooper"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	done := make(chan struct{})

	// goroutine to recieve messages and print them
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}

			// first try to unmarshal the whole thing to json and pretty print
			var obj map[string]interface{}
			err = json.Unmarshal(message, &obj)
			if err == nil {
				m, _ := json.MarshalIndent(obj, ">> ", "  ")
				log.Printf(">> %s\n-----", m)
			} else {
				// if that doesn't work, try to parse as url parameters and print well known ones as json
				u, err := url.ParseQuery(string(message))
				if err == nil {
					for key, query := range u {
						if key == "attachments" {
							var jsonArray []interface{}
							err = json.Unmarshal([]byte(query[0]), &jsonArray)
							if err == nil {
								m, _ := json.MarshalIndent(jsonArray, ">> ", "  ")
								log.Printf(">> %s: \n>> %s\n", key, m)
							} else {
								log.Printf("cound not unmarshal attachments field: %v.\n", err)
							}
						} else {
							for _, q := range query {
								log.Printf(">> %s: %s", key, q)
							}
						}
					}
				} else {
					// if that doesn't work, just print it raw
					log.Printf(">> %s\n-----", message)
				}
			}
		}
	}()

	input := make(chan []byte)
	// goroutine to send messages
	go func() {
		for {
			reader := bufio.NewReader(os.Stdin)
			cmdString, err := reader.ReadString('\n')
			cmdString = strings.TrimSpace(cmdString)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			// if cmd starts with magic sequence, it's a message
			if strings.HasPrefix(cmdString, "<< ") {
				m := &Message{Type: "message", Channel: "C2147483705", User: "U2147483697", Text: trimLeftChars(cmdString, 3), Ts: "1355517523.000005"}
				b, err := json.Marshal(m)
				if err != nil {
					log.Printf("error marshalling string into message: %v.\n", err)
				} else {
					input <- b
				}
			} else {
				input <- []byte(cmdString)
			}
		}
	}()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case message := <-input:
			err := c.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("error write:", err)
				return
			}
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
func startKubectlPortForward(kubeSvc *string, kubePortFwd arrayFlags) *os.Process {
	cmd := exec.Command("kubectl", "port-forward", "service/"+*kubeSvc)
	cmd.Args = append(cmd.Args, kubePortFwd...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanWords)
	err := cmd.Start()
	var localPorts []string
	for _, arg := range kubePortFwd {
		localPorts = append(localPorts, strings.Split(arg, ":")[0])
	}
	count := len(localPorts)
	var found bool
	for scanner.Scan() {
		m := scanner.Text()
		for _, port := range localPorts {
			if strings.Contains(m, port) {
				count = count - 1
			}
			if count == 0 {
				found = true
			}
		}
		if found {
			log.Println("Port-forward up and running. continuing...")
			break
		}
	}
	if err != nil {
		log.Fatalf("failed to launch kubectl: %v.\n", err)
	}
	return cmd.Process
}
func trimLeftChars(s string, n int) string {
	m := 0
	for i := range s {
		if m >= n {
			return s[i:]
		}
		m++
	}
	return s[:0]
}
