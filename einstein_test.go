package main

import (
	"reflect"
	"testing"
)

// expectedEinstein builds the unique solution as a single expression:
// (german
//
//	(one   norwegian yellow cats   water  dunhill)
//	(two   dane      blue   horse  tea    blend)
//	(three brit      red    birds  milk   pall-mall)
//	(four  german    green  fish   coffee prince)
//	(five  swede     white  dog    beer   bluemaster))
func expectedEinstein() expression {
	row := func(p, n, c, pet, d, s int) expression {
		return list(number(p), number(n), number(c), number(pet), number(d), number(s))
	}
	street := list(
		row(posOne, natNorwegian, colYellow, petCats, drnWater, smkDunhill),
		row(posTwo, natDane, colBlue, petHorse, drnTea, smkBlend),
		row(posThree, natBrit, colRed, petBirds, drnMilk, smkPallMall),
		row(posFour, natGerman, colGreen, petFish, drnCoffee, smkPrince),
		row(posFive, natSwede, colWhite, petDog, drnBeer, smkBluemaster),
	)
	return list(number(natGerman), street)
}

func TestEinstein(t *testing.T) {
	got := run(callfresh(einsteinGoal))
	want := []expression{expectedEinstein()}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Einstein:\ngot  %v\nwant %v", got, want)
	}
}

func BenchmarkEinstein(b *testing.B) {
	for b.Loop() {
		run(callfresh(einsteinGoal))
	}
}

func BenchmarkEinsteinSync(b *testing.B) {
	synchronous = true
	defer func() { synchronous = false }()
	for b.Loop() {
		run(callfresh(einsteinGoal))
	}
}

func TestEinsteinSync(t *testing.T) {
	synchronous = true
	defer func() { synchronous = false }()
	got := run(callfresh(einsteinGoal))
	want := []expression{expectedEinstein()}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Einstein (sync):\ngot  %v\nwant %v", got, want)
	}
}
