package util

import (
	"sort"
)

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// indexedByte can be considered an implementation detail of Stretch
type indexedByte struct {
	Idx  int
	Char rune
}

// String returns the indexedByte's Char field as a 1-byte string
func (ib indexedByte) String() string {
	return string([]rune{ib.Char})
}

// Stretch stretches a string out so it is exactly toLength characters in length
func Stretch(s string, toLength uint8) string {
	runeSet := []rune(s)
	l := len(runeSet)
	if l > 255 {
		return s[:256]
	}

	chars := [256]indexedByte{}
	for i := range chars {
		chars[i] = indexedByte{Idx: i % l, Char: runeSet[i%l]}
	}

	sort.Slice(chars[:], func(i, j int) bool {
		return chars[i].Idx < chars[j].Idx
	})

	res := ""
	for _, iChar := range chars {
		res += iChar.String()
	}

	return res
}
