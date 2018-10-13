package dicelang

import (
	"testing"
)

func benchmarkSimpleParse(cmd string, b *testing.B) {
	for n := 0; n < b.N; n++ {
		var p *Parser
		p = NewParser(cmd)
		_, root, _ := p.Statements()
		root.GetDiceSet()
	}
}
func BenchmarkSimpleParse1(b *testing.B)       { benchmarkSimpleParse("roll 1d20", b) }
func BenchmarkSimpleParse4d6Lx10(b *testing.B) { benchmarkSimpleParse("roll 5d20", b) }
func BenchmarkSimpleParse20d20x10(b *testing.B) {
	benchmarkSimpleParse("roll 20d20 blue and twenty d10 red", b)
}
