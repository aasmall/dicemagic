package main

import (
	"strings"
)

type Attack struct {
	DamageSegment []DamageSegment
}

type DamageSegment struct {
	diceExpression string
	diceResult     int64
	damagetype     string
	operator       string
}

func (a *Attack) totalDamage() map[string]int64 {
	m := make(map[string]int64)
	a.rollAttack()
	for _, e := range a.DamageSegment {
		m[strings.Title(e.damagetype)] += e.diceResult
	}
	return m
}
func (d *DamageSegment) rollDamage() {
	d.diceResult = evaluate(parse(d.diceExpression))
}

func (a *Attack) rollAttack() {
	for i, _ := range a.DamageSegment {
		a.DamageSegment[i].rollDamage()
	}
}
