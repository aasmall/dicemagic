package main

import (
        //"fmt"
        "testing"
)

func TestNaturalLanguageParsing(t *testing.T) {
        expression := "<@UAA6PDK7S> roll 1d4+12"
        result := parseMentionAndRoll(expression)
        if result <= 10 {
                t.Fatalf("failed to roll dice.")
        }
}
