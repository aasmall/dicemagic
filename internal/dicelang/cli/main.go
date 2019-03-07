package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/aasmall/dicemagic/internal/dicelang"
	"github.com/aasmall/dicemagic/internal/dicelang/errors"
)

func main() {
	var path, cmd string
	var verbose, prob bool
	flag.StringVar(&path, "path", "", "Path to a file with one roll command per line.")
	flag.StringVar(&cmd, "cmd", "roll 1d20 rep 5", "Roll command")
	flag.BoolVar(&verbose, "v", false, "Display ast for each statement")
	flag.BoolVar(&prob, "p", false, "Display probability map for each statement")
	flag.Parse()
	if path == "" {
		fmt.Println(cmd)
		printDiceInfo(cmd, verbose, prob)
	} else {
		c := make(chan string)
		go readRollsFromFile(c, path)
		for cmd := range c {
			fmt.Println(cmd)
			printDiceInfo(cmd, verbose, prob)
		}
	}
}

func sortProbMap(m map[int64]float64) []int64 {
	var keys []int64
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func printDiceInfo(cmd string, verbose bool, prob bool) {
	var p *dicelang.Parser
	p = dicelang.NewParser(cmd)
	root, err := p.Statements()
	if err != nil {
		fmt.Println(err.Error(), err.(*errors.LexError).Col, err.(*errors.LexError).Line)
		return
	}
	//fmt.Printf("Statement %d\n", i+1)
	total, diceSet, err := root.GetDiceSet()
	if err != nil {
		fmt.Printf("Could not parse input: %v\n", err)
		return
	}
	if verbose {
		fmt.Print("AST:\n----------")
		dicelang.PrintAST(root, 0)
		fmt.Print("\n----------")

	}
	if prob {
		for _, v := range diceSet.Dice {
			probMap := dicelang.DiceProbability(v.Count, v.Sides, v.DropHighest, v.DropLowest)
			keys := sortProbMap(probMap)
			fmt.Printf("\nProbability Map for %+v:\n", v)
			for _, k := range keys {
				fmt.Printf("%2d:  %2.5F%%\n", k, probMap[k])
			}
			fmt.Print("----------\n")
		}
	}
	fmt.Printf("Total: %+v\n", total)
	fmt.Printf("Color Map: %+v\n", diceSet.TotalsByColor)
	//pre := dicelang.ReStringAST(stmt)
	pre, _ := root.String()
	fmt.Println(pre)
	fmt.Println("----------")
	fmt.Println(fmt.Printf("%+v", diceSet))
	fmt.Println("----------")

}

func readRollsFromFile(c chan string, path string) {
	defer close(c)
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Could not open file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		c <- scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Could not scan file: %v\n", err)
		return
	}
}
