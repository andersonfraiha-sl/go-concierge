package regras

import "time"

type diaDoAno struct {
	mes time.Month
	dia int
}

var feriadosFixos = map[diaDoAno]bool{
	{time.January, 1}: true, {time.April, 21}: true, {time.May, 1}: true,
	{time.September, 7}: true, {time.October, 12}: true, {time.November, 2}: true,
	{time.November, 15}: true, {time.November, 20}: true, {time.December, 25}: true,
}

// Dias relativos à Páscoa: Carnaval (segunda e terça), Sexta-feira Santa, Corpus Christi.
var feriadosMoveis = [...]int{-48, -47, -2, 60}

// FeriadoNacional diz se o dia civil de d (no fuso de d) é feriado nacional.
func FeriadoNacional(d time.Time) bool {
	ano, mes, dia := d.Date()
	if feriadosFixos[diaDoAno{mes, dia}] {
		return true
	}
	hoje := time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)
	pascoa := pascoaDe(ano)
	for _, delta := range feriadosMoveis {
		if pascoa.AddDate(0, 0, delta).Equal(hoje) {
			return true
		}
	}
	return false
}

// pascoaDe usa o algoritmo gregoriano anônimo (Meeus/Jones/Butcher).
func pascoaDe(ano int) time.Time {
	a, b, c := ano%19, ano/100, ano%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	mes := (h + l - 7*m + 114) / 31
	dia := (h+l-7*m+114)%31 + 1
	return time.Date(ano, time.Month(mes), dia, 0, 0, 0, 0, time.UTC)
}
