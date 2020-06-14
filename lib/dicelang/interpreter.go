package dicelang

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	errors "github.com/aasmall/dicemagic/lib/dicelang-errors"
)

//PrintAST prints a formatted version of the ast to a string
func PrintAST(t *AST, identation int) string {
	var b bytes.Buffer
	b.WriteRune('\n')
	for i := 0; i < identation; i++ {
		b.WriteString(" ")
	}
	b.WriteRune('(')
	b.WriteString(t.Sym + ":" + t.Value)
	if len(t.Children) > 0 {
		for _, c := range t.Children {
			b.WriteString(" ")
			b.WriteString(PrintAST(c, identation+4))

		}
	}
	b.WriteRune(')')
	return b.String()
}

//ReStringAST is a modified inverse shunting yard which converts an AST back to an infix expression.
func ReStringAST(t *AST) string {
	var s, post Stack
	var buff bytes.Buffer
	ch := make(chan *AST)
	go func() {
		emitTokens(ch, t)
		close(ch)
	}()
	for token := range ch {
		fmt.Printf(token.Sym + ":" + token.Value + "\n")
		switch token.Sym {
		case "-":
			//fucking unary operators
			if len(token.Children) == 1 {
				shuntUnary(token, &s)
			} else {
				shuntBinary(token, &s, " ")
			}
		case "+", "*", "^", "/", "<", ">", ">=", "<=":
			shuntBinary(token, &s, " ")
		case "-L", "-H":
			//infix no space, no paren, not worth a function
			op1 := s.Pop().(*AST)
			op2 := s.Pop().(*AST)
			s.Push(&AST{
				Value:        fmt.Sprintf("%s%s%s", op2.Value, token.Value, op1.Value),
				Sym:          token.Sym,
				BindingPower: token.BindingPower})
		case "d":
			//infix dice
			op1 := s.Pop().(*AST)
			op2 := s.Pop().(*AST)
			s.Push(&AST{
				Value:        fmt.Sprintf("%s%s%s(%%s)", op2.Value, token.Value, op1.Value),
				Sym:          token.Sym,
				BindingPower: token.BindingPower})
		case "(NUMBER)":
			//operand
			s.Push(token)
		case "(IDENT)":
			//postfix
			post.Push(token)
		case "{":
			buff.WriteString(token.Value + " ")
			post.Push(&AST{Value: "}"})
		default:
			//prefix
			buff.WriteString(token.Value + " ")
		}
	}
	for !s.Empty() {
		buff.WriteString(", " + s.Pop().(*AST).Value)
	}
	for !post.Empty() {
		buff.WriteString(" " + post.Pop().(*AST).Value)
	}
	//fmt.Printf("postStack:" + stringPostfix(&post))

	return "\n" + buff.String()
}

func shuntBinary(token *AST, s *Stack, spacer string) {
	op1 := s.Pop().(*AST)
	op2 := s.Pop().(*AST)
	if token.BindingPower > op1.BindingPower {
		s.Push(&AST{
			Value:        fmt.Sprintf("(%s%s%s%s%s)", op2.Value, spacer, token.Value, spacer, op1.Value),
			Sym:          "(COMPOUND)",
			BindingPower: token.BindingPower})
	} else {
		s.Push(&AST{
			Value:        fmt.Sprintf("%s%s%s%s%s", op2.Value, spacer, token.Value, spacer, op1.Value),
			Sym:          "(COMPOUND)",
			BindingPower: token.BindingPower})
	}
}
func shuntUnary(token *AST, s *Stack) {
	op1 := s.Pop().(*AST)
	s.Push(&AST{
		Value:        fmt.Sprintf("%s%s", token.Value, op1.Value),
		Sym:          "(COMPOUND)",
		BindingPower: token.BindingPower})

}

func emitTokens(ch chan *AST, t *AST) {
	if len(t.Children) > 0 {
		for _, c := range t.Children {
			emitTokens(ch, c)
		}
	}
	ch <- t
}

// Convert AST to Infix expression
func (token *AST) String() (string, error) {
	var buf bytes.Buffer
	var preStack, postStack, s, reverse Stack
	if len(token.Children) > 0 {
		err := token.inverseShuntingYard(&buf, &preStack, &postStack, &s, "", 0)
		if err != nil {
			return "", errors.NewDicelangError(err.Error(), errors.InvalidAST, err)
		}
		for !preStack.Empty() {
			reverse.Push(preStack.Pop())
		}
		for !reverse.Empty() {
			buf.WriteString(reverse.Pop().(*AST).Value + " ")
		}
		for !s.Empty() {
			reverse.Push(s.Pop())
		}
		for !reverse.Empty() {
			buf.WriteString(reverse.Pop().(*AST).Value + " ")
		}
		for !postStack.Empty() {
			reverse.Push(postStack.Pop())
		}
		for !reverse.Empty() {
			buf.WriteString(reverse.Pop().(*AST).Value + " ")
		}
	}
	return strings.TrimSpace(buf.String()), nil
}

// GLWT Public License
// Copyright (c) Everyone, except Author
//
// The author has absolutely no clue what the code in this function does.
// It might just work or not, there is no third option.
//
// Everyone is permitted to copy, distribute, modify, merge, sell, publish,
// sublicense or whatever they want with this function but at their OWN RISK.
//
//
//                 GOOD LUCK WITH THAT PUBLIC LICENSE
//    TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION, AND MODIFICATION
//
// 0. You just DO WHATEVER YOU WANT TO as long as you NEVER LEAVE A
// TRACE TO TRACK THE AUTHOR of the original product to blame for or held
// responsible.
//
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
// WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE FUNCTION OR THE USE OR OTHER DEALINGS IN THE FUNCTION.
//
// Good luck and Godspeed.
func (token *AST) inverseShuntingYard(buff *bytes.Buffer, preStack *Stack, postStack *Stack, s *Stack, lastSym string, childNum int) error {
	if len(token.Children) > 0 {
		for i, c := range token.Children {
			err := c.inverseShuntingYard(buff, preStack, postStack, s, token.Sym, i)
			if err != nil {
				return err
			}
			if s.Top() == nil {
				return errors.New("Invalid AST. Cannot convert to infix expression")
			}
			if s.Top().(*AST).Sym == "(COMPOUND)" {
				for !postStack.Empty() {
					left := s.Pop().(*AST)
					s.Push(&AST{
						Value:        fmt.Sprintf("%s %s", left.Value, postStack.Pop().(*AST).Value),
						Sym:          "(COMPOUND)",
						BindingPower: token.BindingPower})
				}
			}
			for !preStack.Empty() {
				right := s.Pop().(*AST)
				pre := preStack.Pop().(*AST)
				s.Push(&AST{
					Value:        fmt.Sprintf("%s %s", pre.Value, right.Value),
					Sym:          "(COMPOUND)",
					BindingPower: token.BindingPower})
			}
		}
	}
	switch sym := strings.ToUpper(token.Sym); sym {
	case "-":
		//fucking unary operators
		if len(token.Children) == 1 {
			shuntUnary(token, s)
		} else {
			shuntBinary(token, s, " ")
		}
	case "+", "*", "^", "/", "<", ">", ">=", "<=":
		shuntBinary(token, s, " ")
	case "-L", "-H":
		//binary no space, no paren, not worth a function
		op1 := s.Pop().(*AST)
		op2 := s.Pop().(*AST)
		s.Push(&AST{
			Value:        fmt.Sprintf("%s%s%s", op2.Value, token.Value, op1.Value),
			Sym:          token.Sym,
			BindingPower: token.BindingPower})
	case "D":
		//infix dice
		op1 := s.Pop().(*AST)
		op2 := s.Pop().(*AST)
		var sym string
		if lastSym == "D" || lastSym == "d" {
			sym = "d"
		} else {
			sym = "(COMPOUND)"
		}
		s.Push(&AST{
			Value:        fmt.Sprintf("%s%s%s(%%s)", op2.Value, "d", op1.Value),
			Sym:          sym,
			BindingPower: token.BindingPower})
	case "REP":
		op1 := s.Pop().(*AST)
		op2 := s.Pop().(*AST)
		sym = "(COMPOUND)"
		var b [][]byte

		reps, _ := strconv.Atoi(op1.Value)
		for index := 0; index < reps; index++ {
			b = append(b, []byte(op2.Value))
		}
		compoundValue := string(bytes.Join(b, []byte(", ")))
		s.Push(&AST{
			Value:        compoundValue,
			Sym:          sym,
			BindingPower: token.BindingPower})
	case "(NUMBER)":
		//operand
		s.Push(token)
	case "(IDENT)":
		//postfix
		token.Value = strings.Title(token.Value)
		postStack.Push(token)
	case "{":
		preStack.Push(token)
		if lastSym == "if" && childNum == 1 {
			postStack.Push(&AST{Value: "else"})
		}
		postStack.Push(&AST{Value: "}"})
	case "IF":
		preStack.Push(token)
	case "(ROOTNODE)":
	default:
		//prefix
		token.Value = strings.Title(strings.ToLower(token.Value))
		preStack.Push(token)
	}
	return nil
}

func (token *AST) eval(ds *DiceSet) (float64, *DiceSet, error) {
	switch strings.ToUpper(token.Sym) {
	case "(NUMBER)":
		i, _ := strconv.ParseFloat(token.Value, 64)
		if len(token.Children) > 0 {
			//grab any color below, get it on ds
			token.Children[0].eval(ds)
		}
		return i, ds, nil
	case "-H", "-L":
		var sum, z float64
		var err error

		for _, c := range token.Children {
			z, ds, err = c.eval(ds)
			if err != nil {
				return 0, ds, err
			}
			sum += z
		}
		switch token.Sym {
		case "-H":
			ds.DropHighest = int64(sum)
		case "-L":
			ds.DropLowest = int64(sum)
		}
		return 0, ds, nil
	case "D":
		dice := &Dice{}
		var nums []int64
		for i := 0; i < len(token.Children); i++ {
			var num float64
			var err error
			num, ds, err = token.Children[i].eval(ds)
			if err != nil {
				return 0, nil, err
			}
			nums = append(nums, int64(num))

		}
		dice.Count = nums[0]
		dice.Sides = nums[1]
		//actually roll dice here
		res, err := ds.PushAndRoll(dice)

		return float64(res), ds, err
	case "+", "-", "*", "/", "^":
		x, ds, err := token.preformArithmitic(ds, token.Sym)
		if err != nil {
			return 0, ds, err
		}
		return x, ds, nil
	case "{", "ROLL", "(ROOTNODE)":
		var x float64
		for _, c := range token.Children {
			y, ds, err := c.eval(ds)
			if err != nil {
				return 0, ds, err
			}
			x += y
		}
		return x, ds, nil
	case "(IDENT)":
		ds.PushColor(token.Value)
		return 0, ds, nil
	case "REP":
		numberOfReps, _, _ := token.Children[1].eval(ds)
		var x float64
		for index := 0; index < int(numberOfReps); index++ {
			y, ds, err := token.Children[0].eval(ds)
			if err != nil {
				return 0, ds, err
			}
			x += y
		}
		return x, ds, nil
	case "IF":
		res, ds, err := token.Children[0].evaluateBoolean(ds)
		if err != nil {
			return 0, ds, err
		}
		fmt.Print(res, " ")
		var c *AST
		if res {
			c = token.Children[1]
		} else {
			if len(token.Children) < 3 {
				return 0, ds, nil
			}
			c = token.Children[2]
		}
		var x float64
		//Evaluate chosen child
		y, ds, err := c.eval(ds)
		if err != nil {
			return 0, ds, err
		}
		x += y
		return x, ds, nil
	default:
		return 0, ds, fmt.Errorf("unsupported symbol: %s", token.Sym)
	}
}

func (token *AST) preformArithmitic(ds *DiceSet, op string) (float64, *DiceSet, error) {
	//arithmitic is always binary
	//...except for the "-" unary operator
	diceCount := len(ds.Dice)
	var nums []float64
	ds.ColorDepth++
	for _, c := range token.Children {
		switch c.Sym {
		case "(IDENT)":
			_, ds, err := c.eval(ds)
			if err != nil {
				return 0, ds, err
			}
		default:
			x, ds, err := c.eval(ds)
			if err != nil {
				return 0, ds, err
			}
			nums = append(nums, x)
		}
	}
	ds.ColorDepth--
	newDice := len(ds.Dice) - diceCount
	var x float64
	switch op {
	case "+":
		for _, y := range nums {
			x += y
		}
	case "-":
		if len(nums) < 2 {
			x = -nums[0]
		} else {
			x = nums[0]
			for i := 1; i < len(nums); i++ {
				x -= nums[i]
			}
		}
	case "*":
		x = nums[0]
		for i := 1; i < len(nums); i++ {
			x *= nums[i]
		}
	case "/":
		x = nums[0]
		for i := 1; i < len(nums); i++ {
			x /= nums[i]
		}
	case "^":
		x = nums[0]
		for i := 1; i < len(nums); i++ {
			x = math.Pow(x, nums[i])
		}
	default:
		return 0, ds, fmt.Errorf("invalid operator: %s", op)
	}
	if len(ds.Colors) > 1 {
		return 0, ds, fmt.Errorf("cannot preform aritimitic on different color dice, try \",\" or \"and\" instead")
	}
	if ds.ColorDepth == 0 {
		color := ds.PopColor()
		for i := 0; i < newDice; i++ {
			ds.Top(i).Color = color
		}
		ds.AddToColor(color, x)
	}

	return x, ds, nil
}
func (token *AST) evaluateBoolean(ds *DiceSet) (bool, *DiceSet, error) {
	left, ds, err := token.Children[0].eval(ds)
	if err != nil {
		return false, ds, err
	}
	right, ds, err := token.Children[1].eval(ds)
	if err != nil {
		return false, ds, err
	}
	switch token.Sym {
	case ">":
		return left > right, ds, nil
	case "<":
		return left < right, ds, nil
	case "<=":
		return left <= right, ds, nil
	case ">=":
		return left >= right, ds, nil
	case "==":
		return left == right, ds, nil
	case "!=":
		return left != right, ds, nil
	}
	return false, ds, fmt.Errorf("bad bool")
}

//PushAndRoll adds a dice roll to the "stack" applying any values from the set
func (d *DiceSet) PushAndRoll(dice *Dice) (int64, error) {
	if d.ColorDepth == 0 {
		dice.Color = d.PopColor()
	}
	dice.DropHighest = d.DropHighest
	dice.DropLowest = d.DropLowest
	d.DropLowest = 0
	d.DropHighest = 0
	res, err := dice.Roll()
	if err != nil {
		return 0, err
	}
	d.Dice = append(d.Dice, dice)
	d.AddToColor(dice.Color, float64(res))
	return res, nil
}

//PushColor pushes a color to the "stack"
func (d *DiceSet) PushColor(color string) {
	d.Colors = append(d.Colors, color)
}

//PeekColor returns the most recently added color from the "stack"
func (d *DiceSet) PeekColor() string {
	if len(d.Colors) > 0 {
		color := d.Colors[len(d.Colors)-1]
		return color
	}
	return ""
}

//PopColor pops a color from the "stack"
func (d *DiceSet) PopColor() string {

	if len(d.Colors) > 0 {
		color := d.Colors[len(d.Colors)-1]
		d.Colors = d.Colors[:len(d.Colors)-1]
		return color
	}
	return ""
}

//Top returns a pointer to the most recently added dice roll
func (d *DiceSet) Top(loc int) *Dice {
	if len(d.Dice) > 0 {
		return d.Dice[len(d.Dice)-loc-1]
	}
	return nil
}

//AddToColor increments the total result for a given color
func (d *DiceSet) AddToColor(color string, value float64) {
	if d.TotalsByColor == nil {
		d.TotalsByColor = make(map[string]float64)
	}
	if d.ColorDepth == 0 {
		d.TotalsByColor[color] += value
	}
}

//Roll rolls the dice, sets Min, Max, and Faces. Returns the total. Can be called multiple times and returns the same value each time.
func (d *Dice) Roll() (int64, error) {
	if d.Total != 0 {
		return d.Total, nil
	}
	faces, result, err := roll(d.Count, d.Sides, d.DropHighest, d.DropLowest)
	if err != nil {
		return 0, err
	}
	if d.DropHighest > 0 || d.DropLowest > 0 {
		d.Min = d.Count - (d.DropHighest + d.DropLowest)
		d.Max = (d.Count - (d.DropHighest + d.DropLowest)) * d.Sides
	} else {
		d.Min = d.Count
		d.Max = d.Count * d.Sides
	}
	d.Faces = faces
	d.Total = result
	return result, nil
}

//Roll creates a random number that represents the roll of
//some dice
func roll(numberOfDice int64, sides int64, H int64, L int64) ([]int64, int64, error) {
	var faces []int64
	if numberOfDice > 1000 {
		return faces, 0, errors.NewDicelangError("I can't hold that many dice!", errors.Friendly, nil)
	} else if sides > 1000 {
		return faces, 0, errors.NewDicelangError("A die with that many sides is basically round", errors.Friendly, nil)
	} else if sides < 1 {
		return faces, 0, errors.NewDicelangError("/me ponders the meaning of a zero sided die", errors.Friendly, nil)
	} else {
		total := int64(0)
		for i := int64(0); i < numberOfDice; i++ {
			face, err := generateRandomInt(1, int64(sides))
			if err != nil {
				return faces, 0, err
			}
			faces = append(faces, face)
			total += face
		}
		sort.Slice(faces, func(i, j int) bool { return faces[i] < faces[j] })
		if H > 0 {
			keptFaces := faces[:int64(len(faces))-H]
			total = sumInt64(keptFaces...)
		} else if L > 0 {
			keptFaces := faces[L:]
			total = sumInt64(keptFaces...)
		}
		return faces, total, nil
	}
}

func generateRandomInt(min int64, max int64) (int64, error) {
	if max <= 0 || min < 0 {
		err := fmt.Errorf("cannot make a random int of size zero")
		return 0, err
	}
	size := max - min
	if size == 0 {
		return 1, nil
	}
	//rand.Int does not return the max value, add 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(size+1)))
	if err != nil {
		err = fmt.Errorf("couldn't make a random number. Out of entropy?")
		return 0, err
	}
	n := nBig.Int64()
	return n + int64(min), nil
}

func sumInt64(nums ...int64) int64 {
	r := int64(0)
	for _, n := range nums {
		r += n
	}
	return r
}
