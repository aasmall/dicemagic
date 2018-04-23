package main

import (
        "fmt"
        //"context"
        "testing"
)

func TestNaturalLanguageParsing(t *testing.T) {
        expression := "<@UAA6PDK7S> roll 1d4+12"
        result := parseMentionAndRoll(expression)
        if result <= 12 {
                t.Fatalf("failed to roll dice.")
        }
}
func TestComplexAttack(t *testing.T) {
        naturalLanguageAttack := "<@UAA6PDK7S> roll 1d4+1(Mundane)+1d8+5(Fire)-1d4(necrotic)"

        var attack = parseLanguageintoAttack(nil, naturalLanguageAttack)
        attack.totalDamage()
        fmt.Println(fmt.Sprintf("Attack: %+v", attack.totalDamage()))
        for _, e := range attack.DamageSegment {
                if !(e.diceResult > 0) {
                        t.Fatalf("Internal damage Segements not updating when rolling.")
                }
        }
}
