package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"strconv"
    "google.golang.org/appengine"
    "regexp"
)

const UintBytes = 2
var diceRegexp = regexp.MustCompile(`(?i)^\/(\d+)d(\d+)$`)

func main() {
	//minPtr := flag.Int("min", 1, "min value")
	//maxPtr := flag.Int("max", 4, "max value")
	//flag.Parse()
	//fmt.Println("I rolled", roll(*minPtr,*maxPtr))
	http.HandleFunc("/", handle)
	appengine.Main()
}
func handle(w http.ResponseWriter, r *http.Request) { 
	//numberOfDice, sides:=parseDice(r.URL.Path)
	content := r.URL.Path
	if !diceRegexp.MatchString(content) {
		fmt.Fprintf(w,"%s is not a valid roll\n", strings.Replace(content, "/","",-1))
		return
	}
	numberOfDice,_:=strconv.ParseInt(diceRegexp.FindStringSubmatch(content)[1],10,0)
	sides,_:=strconv.ParseInt(diceRegexp.FindStringSubmatch(content)[2],10,0)
	fmt.Fprintln(w, roll(int(numberOfDice), int(sides)))
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
