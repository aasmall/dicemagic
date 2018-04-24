package main

import (
	"crypto/rand"
	"fmt"
	"google.golang.org/appengine"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var diceRegexp = regexp.MustCompile(`(?i)^(\d+)d(\d+)$`)

func main() {
	http.HandleFunc("/", rootHandle)
	http.HandleFunc("/roll/", handle)
	http.HandleFunc("/slack/roll/", slackRoll)
	http.HandleFunc("/slack/events/", slackEventRouter)
	http.HandleFunc("/slack/oauth/", slackOauthHandler)
	http.HandleFunc("/dflow/", dialogueWebhookHandler)
	appengine.Main()
}
func handle(w http.ResponseWriter, r *http.Request) {
	expression := r.URL.Path[1:]
	result := evaluate(parse(expression))
	fmt.Fprintf(w, "%d\n", result)
}

func GenerateRandomInt(min int, max int) int64 {
	size := max - min + 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(size)))
	if err != nil {
		panic(err)
	}
	n := nBig.Int64()
	return n + int64(min)
}
func roll(numberOfDice int, sides int) int64 {
	result := int64(0)
	for i := 0; i < numberOfDice; i++ {
		x := GenerateRandomInt(1, sides)
		result += x
	}
	return result
}

type ExpressionNode struct {
	expression string
	left       *ExpressionNode
	right      *ExpressionNode
}

// Create a tree of binary operations to execute
func parse(expression string) ExpressionNode {
	node := ExpressionNode{}
	sides := make([]string, 0)
	if strings.Contains(expression, "+") {
		node.expression = "+"
		sides = strings.SplitN(expression, "+", 2)
	} else if strings.Contains(expression, "-") {
		node.expression = "-"
		sides = strings.SplitN(expression, "-", 2)
	} else if strings.Contains(expression, "*") {
		node.expression = "*"
		sides = strings.SplitN(expression, "*", 2)
	} else if strings.Contains(expression, "/") {
		node.expression = "/"
		sides = strings.SplitN(expression, "/", 2)
	} else {
		node.expression = expression
		return node
	}
	left := parse(sides[0])
	right := parse(sides[1])
	node.left = &left
	node.right = &right
	return node
}

// Evaluate a tree of binary operations
func evaluate(node ExpressionNode) int64 {
	if node.expression == "+" {
		return evaluate(*node.left) + evaluate(*node.right)
	}
	if node.expression == "-" {
		return evaluate(*node.left) - evaluate(*node.right)
	}
	if node.expression == "*" {
		return evaluate(*node.left) * evaluate(*node.right)
	}
	if node.expression == "/" {
		return evaluate(*node.left) / evaluate(*node.right)
	}
	if diceRegexp.MatchString(node.expression) {
		numberOfDice, _ := strconv.ParseInt(diceRegexp.FindStringSubmatch(node.expression)[1], 10, 0)
		sides, _ := strconv.ParseInt(diceRegexp.FindStringSubmatch(node.expression)[2], 10, 0)
		rollResult := roll(int(numberOfDice), int(sides))
		return rollResult
	} else {
		number, _ := strconv.ParseInt(node.expression, 10, 0)
		return number
	}
}
