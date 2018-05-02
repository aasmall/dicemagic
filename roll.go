package main

import (
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/appengine"
)

var diceRegexp = regexp.MustCompile(`(?i)^(\d+)d(\d+)$`)

func main() {
	http.HandleFunc("/", rootHandle)
	http.HandleFunc("/slack/roll/", slackRoll)
	http.HandleFunc("/dflow/", dialogueWebhookHandler)
	appengine.Main()
}

type expressionNode struct {
	expression string
	left       *expressionNode
	right      *expressionNode
}

// Create a tree of binary operations to execute
func parse(expression string) expressionNode {
	node := expressionNode{}
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
/*
func evaluate(node expressionNode) int64 {
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
		rollResult, _ := roll(int(numberOfDice), int(sides))
		return rollResult
	} else {
		number, _ := strconv.ParseInt(node.expression, 10, 0)
		return number
	}
}
*/
