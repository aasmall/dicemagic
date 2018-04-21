package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"strconv"
    "google.golang.org/appengine"
)

const UintBytes = 2

func main() {
	//minPtr := flag.Int("min", 1, "min value")
	//maxPtr := flag.Int("max", 4, "max value")
	//flag.Parse()
	//fmt.Println("I rolled", roll(*minPtr,*maxPtr))
	http.HandleFunc("/", handle)
	appengine.Main()
}
func handle(w http.ResponseWriter, r *http.Request) { 
	numberOfDice, sides:=parseDice(r.URL.Path)
	fmt.Fprintln(w, roll(numberOfDice, sides))
}
func parseDice(s string) (int, int){

	returnString:=strings.Replace(s,"/","",-1)

	values:=strings.Split(returnString,"d")
	numberOfDice:=values[0]
	sides:=values[1]
	intDice, err:=strconv.ParseInt(numberOfDice,10,0)
	if intDice==0 {
		intDice=1
	}
	intSides, err:=strconv.ParseInt(sides,10,0)

	if err != nil {
		panic(err)
	}
	return int(intDice), int(intSides)

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
