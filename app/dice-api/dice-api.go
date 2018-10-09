package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"

	"cloud.google.com/go/logging"
	"github.com/aasmall/dicemagic/app/dicelang"
)

func main() {
	http.HandleFunc("/dice-api", DiceAPIHandler)

	ctx := context.Background()

	// Sets your Google Cloud Platform project ID.
	projectID := "k8s-dice-magic"
	redirectURL := "https://www.smallnet.org/"

	// Creates a client.
	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Sets the name of the log to write to.
	logName := "dicemagic-api"

	Debuglogger := client.Logger(logName).StandardLogger(logging.Debug)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		Debuglogger.Printf("Redirecting to: %s", redirectURL)
		http.Redirect(w, r, redirectURL, 302)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func DiceAPIHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	printDiceInfo(w, q.Get("cmd"), true, true)
	//fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Query)
}

func printDiceInfo(w http.ResponseWriter, cmd string, verbose bool, prob bool) {
	var p *dicelang.Parser
	p = dicelang.NewParser(cmd)
	_, root, err := p.Statements()
	if err != nil {
		fmt.Fprintf(w, err.Error(), err.(dicelang.LexError).Col, err.(dicelang.LexError).Line)
		return
	}
	//fmt.Printf("Statement %d\n", i+1)
	total, diceSet, err := root.GetDiceSet()
	if err != nil {
		fmt.Fprintf(w, "Could not parse input: %v\n", err)
		return
	}
	if verbose {
		fmt.Fprintf(w, "AST:\n----------")
		dicelang.PrintAST(root, 0)
		fmt.Fprintf(w, "\n----------")

	}
	if prob {
		for _, v := range diceSet.Dice {
			probMap := dicelang.DiceProbability(v.Count, v.Sides, v.DropHighest, v.DropLowest)
			keys := sortProbMap(probMap)
			fmt.Fprintf(w, "\nProbability Map for %+v:\n", v)
			for _, k := range keys {
				fmt.Fprintf(w, "%2d:  %2.5F%%\n", k, probMap[k])
			}
			fmt.Fprintf(w, "----------\n")
		}
	}
	fmt.Fprintf(w, "Total: %+v\n", total)
	fmt.Fprintf(w, "Color Map: %+v\n", diceSet.TotalsByColor)
	//pre := dicelang.ReStringAST(stmt)
	pre := root.String()
	fmt.Fprintf(w, pre)
	fmt.Fprintf(w, "----------")

}
func sortProbMap(m map[int64]float64) []int64 {
	var keys []int64
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
