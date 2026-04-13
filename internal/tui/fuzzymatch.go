package tui

import (
	"unicode"
)

const (
	scoreMatch        = 16
	scoreGapStart     = -3
	scoreGapExtension = -1

	bonusBoundary          int16 = 8
	bonusBoundaryWhite     int16 = 8 // path scheme: whitespace same as boundary
	bonusBoundaryDelimiter int16 = 9 // path scheme: delimiters (/ : ; |) get extra bonus
	bonusNonWord           int16 = 8
	bonusCamel123          int16 = 7
	bonusConsecutive       int16 = 4
	bonusFirstCharMult     int16 = 2
)

type charClass int

// Order matters: computeBonus uses > charNonWord to select boundary-eligible characters.
const (
	charWhite charClass = iota
	charNonWord
	charDelimiter
	charLower
	charUpper
	charLetter
	charNumber
	charClassCount
)

var (
	asciiClasses [128]charClass
	bonusMatrix  [charClassCount][charClassCount]int16
)

func init() {
	for i := range 128 {
		ch := rune(i)
		switch {
		case ch >= 'a' && ch <= 'z':
			asciiClasses[i] = charLower
		case ch >= 'A' && ch <= 'Z':
			asciiClasses[i] = charUpper
		case ch >= '0' && ch <= '9':
			asciiClasses[i] = charNumber
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			asciiClasses[i] = charWhite
		case ch == '/' || ch == ':' || ch == ';' || ch == '|':
			asciiClasses[i] = charDelimiter
		default:
			asciiClasses[i] = charNonWord
		}
	}

	for i := range charClassCount {
		for j := range charClassCount {
			bonusMatrix[i][j] = computeBonus(charClass(i), charClass(j))
		}
	}
}

func computeBonus(prev, cur charClass) int16 {
	if cur > charNonWord {
		switch prev {
		case charWhite:
			return bonusBoundaryWhite
		case charDelimiter:
			return bonusBoundaryDelimiter
		case charNonWord:
			return bonusBoundary
		}
	}
	if prev == charLower && cur == charUpper {
		return bonusCamel123
	}
	if prev != charNumber && cur == charNumber {
		return bonusCamel123
	}
	switch cur {
	case charNonWord, charDelimiter:
		return bonusNonWord
	case charWhite:
		return bonusBoundaryWhite
	}
	return 0
}

func classOf(ch rune) charClass {
	if ch < 128 {
		return asciiClasses[ch]
	}
	switch {
	case unicode.IsLower(ch):
		return charLower
	case unicode.IsUpper(ch):
		return charUpper
	case unicode.IsNumber(ch):
		return charNumber
	case unicode.IsLetter(ch):
		return charLetter
	case unicode.IsSpace(ch):
		return charWhite
	}
	return charNonWord
}

func bonusAt(runes []rune, idx int) int16 {
	if idx == 0 {
		return bonusBoundaryDelimiter // path scheme: start treated as after delimiter
	}
	return bonusMatrix[classOf(runes[idx-1])][classOf(runes[idx])]
}

// FuzzyResult holds the outcome of a fuzzy match.
type FuzzyResult struct {
	Score     int
	Positions []int
}

// FuzzyMatch runs an fzf V2-style scoring DP to find the optimal
// fuzzy match of pattern within input. Returns a zero-score result if no match.
func FuzzyMatch(caseSensitive bool, input string, pattern string) FuzzyResult {
	if pattern == "" {
		return FuzzyResult{}
	}

	inputRunes := []rune(input)
	patRunes := []rune(pattern)
	N := len(inputRunes)
	M := len(patRunes)

	if !caseSensitive {
		for i, r := range inputRunes {
			inputRunes[i] = unicode.ToLower(r)
		}
		for i, r := range patRunes {
			patRunes[i] = unicode.ToLower(r)
		}
	}

	// Phase 1: verify all pattern chars exist and find bounds.
	firstOccurrence := make([]int, M)
	pidx := 0
	for i := 0; i < N && pidx < M; i++ {
		if inputRunes[i] == patRunes[pidx] {
			firstOccurrence[pidx] = i
			pidx++
		}
	}
	if pidx != M {
		return FuzzyResult{}
	}

	// Find last occurrence of last pattern char for right bound.
	lastIdx := firstOccurrence[M-1]
	for i := N - 1; i > lastIdx; i-- {
		if inputRunes[i] == patRunes[M-1] {
			lastIdx = i
			break
		}
	}

	minIdx := firstOccurrence[0]
	width := lastIdx - minIdx + 1

	// Precompute bonuses for the search region.
	// Use the original (non-lowercased) runes for classification.
	origRunes := []rune(input)
	B := make([]int16, width)
	for i := range width {
		B[i] = bonusAt(origRunes, minIdx+i)
	}

	// Single-char pattern: find best scoring position.
	if M == 1 {
		bestScore := int16(0)
		bestPos := -1
		for i := range width {
			if inputRunes[minIdx+i] == patRunes[0] {
				s := int16(scoreMatch) + B[i]*bonusFirstCharMult
				if s > bestScore {
					bestScore = s
					bestPos = minIdx + i
					if B[i] >= bonusBoundaryDelimiter {
						break // delimiter boundary is the best possible
					}
				}
			}
		}
		if bestPos < 0 {
			return FuzzyResult{}
		}
		return FuzzyResult{
			Score:     int(bestScore),
			Positions: []int{bestPos},
		}
	}

	// DP scoring matrix.
	// H[i*width + j] = best score for matching pattern[0..i] within input[minIdx..minIdx+j]
	// C[i*width + j] = consecutive match count ending at (i, j)
	H := make([]int16, M*width)
	C := make([]int16, M*width)

	// Fill row 0 (first pattern character).
	prevH := int16(0)
	inGap := false
	for j := range width {
		var gapS int16
		if inGap {
			gapS = prevH + scoreGapExtension
		} else {
			gapS = prevH + scoreGapStart
		}
		if gapS < 0 {
			gapS = 0
		}

		if inputRunes[minIdx+j] == patRunes[0] {
			matchS := int16(scoreMatch) + B[j]*bonusFirstCharMult
			if matchS >= gapS {
				H[j] = matchS
				C[j] = 1
				prevH = matchS
				inGap = false
			} else {
				H[j] = gapS
				C[j] = 0
				prevH = gapS
				inGap = true
			}
		} else {
			H[j] = gapS
			C[j] = 0
			prevH = gapS
			inGap = true
		}
	}

	// Fill rows 1..M-1.
	for i := 1; i < M; i++ {
		row := i * width
		inGap = false

		for j := range width {
			var s1, s2 int16

			// Gap option: extend from left.
			if j > 0 {
				if inGap {
					s2 = H[row+j-1] + scoreGapExtension
				} else {
					s2 = H[row+j-1] + scoreGapStart
				}
			}

			// Match option: diagonal.
			if inputRunes[minIdx+j] == patRunes[i] && j > 0 {
				diag := H[row-width+j-1]
				s1 = diag + scoreMatch

				b := B[j]
				consecutive := C[row-width+j-1] + 1

				if consecutive > 1 {
					fbIdx := j - int(consecutive) + 1
					if fbIdx < 0 {
						fbIdx = 0
					}
					fb := B[fbIdx]
					if b >= bonusBoundary && b > fb {
						consecutive = 1
					} else {
						if bonusConsecutive > b {
							b = bonusConsecutive
						}
						if fb > b {
							b = fb
						}
					}
				}

				if s1+b < s2 {
					s1 += B[j]
					consecutive = 0
				} else {
					s1 += b
				}

				C[row+j] = consecutive
			} else {
				C[row+j] = 0
			}

			inGap = s1 < s2
			score := max(s1, s2, 0)
			H[row+j] = score
		}
	}

	// Find best score in last row.
	lastRow := (M - 1) * width
	maxScore := int16(0)
	maxPos := -1
	for j := range width {
		if H[lastRow+j] > maxScore {
			maxScore = H[lastRow+j]
			maxPos = j
		}
	}
	if maxPos < 0 {
		return FuzzyResult{}
	}

	// Backtrace to find matched positions.
	// Walk backwards from the best score in the last row. At each cell,
	// C[row+j] > 0 means the DP chose "match" here; accept it and step
	// diagonally. Otherwise the cell was a gap; step left.
	// When gap decay zeroes out scores, the C-based walk may fail to place
	// all pattern chars; fall back to the greedy firstOccurrence positions.
	positions := make([]int, M)
	for k := range positions {
		positions[k] = -1
	}
	i := M - 1
	j := maxPos
	for i >= 0 && j >= 0 {
		row := i * width
		if inputRunes[minIdx+j] == patRunes[i] && C[row+j] > 0 {
			positions[i] = minIdx + j
			i--
			j--
		} else {
			j--
		}
	}
	for _, p := range positions {
		if p < 0 {
			copy(positions, firstOccurrence)
			break
		}
	}

	return FuzzyResult{
		Score:     int(maxScore),
		Positions: positions,
	}
}
