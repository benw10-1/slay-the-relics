package api

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/exp/slices"
	"gotest.tools/v3/assert"
)

func TestDecompress(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		output      string
		shouldError bool
	}{
		{
			desc:        "Empty",
			input:       "",
			output:      "",
			shouldError: true,
		},
		{
			desc:        "Single |",
			input:       "|",
			output:      "",
			shouldError: true,
		},
		{
			desc:        "Minimal Valid",
			input:       "||",
			output:      "",
			shouldError: false,
		},
		{
			desc:        "No compression",
			input:       "||I love love slay the relics and slay the spire",
			output:      "I love love slay the relics and slay the spire",
			shouldError: false,
		},
		{
			desc:        "Unused compression",
			input:       "Foo|Bar||I love love slay the relics and slay the spire",
			output:      "I love love slay the relics and slay the spire",
			shouldError: false,
		},
		{
			desc:        "Basic compression",
			input:       "love|slay the||I &0 &0 &1 relics and &1 spire",
			output:      "I love love slay the relics and slay the spire",
			shouldError: false,
		},
		{
			desc:        "Basic compression",
			input:       "love|slay the||I &0 &0 &1 relics and &1 spire",
			output:      "I love love slay the relics and slay the spire",
			shouldError: false,
		},
		{
			desc:        "Small Deck",
			input:       "card|junk||0,1,1,0,2,0;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      "0,1,1,0,2,0;;;card1;junk;x;;card2;junk;y;;card3;junk;z",
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualOutput, err := decompress(tc.input)
			if tc.shouldError {
				assert.Equal(t, true, err != nil)
				_, err = decompressBytes([]byte(tc.input))
				assert.Equal(t, true, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, actualOutput, tc.output)

			unstableOut, err := decompressBytes([]byte(tc.input))
			assert.NilError(t, err)

			assert.Equal(t, string(unstableOut), tc.output)
		})
	}
}

func TestParseCommaDelimitedIntegerArray(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		output      []int
		shouldError bool
	}{
		{
			desc:        "Empty",
			input:       "",
			output:      []int{},
			shouldError: false,
		},
		{
			desc:        "Dash",
			input:       "-",
			output:      []int{},
			shouldError: false,
		},
		{
			desc:        "Invalid",
			input:       "{",
			output:      []int{},
			shouldError: true,
		},
		{
			desc:        "Single",
			input:       "19",
			output:      []int{19},
			shouldError: false,
		},
		{
			desc:        "Many",
			input:       "1,2,3,4,5,6,7,8,9,1,2,3,4,5,6,7,8,9",
			output:      []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualOutput, err := parseCommaDelimitedIntegerArray(tc.input)
			if tc.shouldError {
				assert.Equal(t, true, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, len(actualOutput), len(tc.output))
			for i := range actualOutput {
				assert.Equal(t, actualOutput[i], tc.output[i])
			}
		})
	}
}

func TestReadDelemitedBytesIntArrs(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		output      []int
		shouldError bool
	}{
		{
			desc:        "Empty",
			input:       "",
			output:      []int{},
			shouldError: false,
		},
		{
			desc:        "Dash",
			input:       "-",
			output:      []int{},
			shouldError: false,
		},
		{
			desc:        "Invalid",
			input:       "{",
			output:      []int{},
			shouldError: true,
		},
		{
			desc:        "Single",
			input:       "19",
			output:      []int{19},
			shouldError: false,
		},
		{
			desc:        "Many",
			input:       "1,2,3,4,5,6,7,8,9,1,2,3,4,5,6,7,8,9",
			output:      []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := make([]int, 0, len(tc.output))
			err := readDelimitedBytes([]byte(tc.input), []byte(","), func(val []byte) error {
				valInt, err := strconv.ParseInt(string(val), 10, 64)
				if err != nil {
					return err
				}

				result = append(result, int(valInt))

				return nil
			})
			if tc.shouldError {
				assert.Equal(t, true, err != nil)
				return
			}

			assert.NilError(t, err)

			assert.DeepEqual(t, result, tc.output)
		})
	}
}

func TestReadDelemitedBytesSplitStringMultiCharDelim(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		delim       string
		output      []string
		shouldError bool
	}{
		{
			desc:        "Empty",
			input:       "",
			delim:       ";;",
			output:      []string{},
			shouldError: false,
		},
		{
			desc:        "Dash",
			input:       "-",
			delim:       ";;",
			output:      []string{},
			shouldError: false,
		},
		{
			desc:        "Single",
			input:       "love slay the spire",
			delim:       ";;",
			output:      []string{"love slay the spire"},
			shouldError: false,
		},
		{
			desc:        "Many",
			input:       "slay;;the;;spire;;is;;an;;awesome;;game,and;;I;;love;;it;;because;;it;;is;;an;;interesting;;game",
			delim:       ";;",
			output:      []string{"slay", "the", "spire", "is", "an", "awesome", "game,and", "I", "love", "it", "because", "it", "is", "an", "interesting", "game"},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := make([]string, 0, len(tc.output))
			err := readDelimitedBytes([]byte(tc.input), []byte(tc.delim), func(val []byte) error {
				result = append(result, string(val))

				return nil
			})
			if tc.shouldError {
				assert.Equal(t, true, err != nil)
				return
			}

			assert.NilError(t, err)

			assert.DeepEqual(t, result, tc.output)
		})
	}
}

func TestDecompressDeck(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		output      map[string]int
		shouldError bool
	}{
		{
			desc:        "Empty",
			input:       "",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:        "Invalid card index",
			input:       "card|junk||};;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:        "Negative card index",
			input:       "card|junk||-1;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:        "Out of bounds card index",
			input:       "card|junk||3;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:  "Simple deck",
			input: "card|junk||0,1,1,0,2,0;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output: map[string]int{
				"card1": 3,
				"card2": 2,
				"card3": 1,
			},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualOutput, err := decompressDeck(tc.input)
			if tc.shouldError {
				assert.Equal(t, true, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, len(actualOutput), len(tc.output))
			for i := range actualOutput {
				assert.Equal(t, actualOutput[i], tc.output[i])
			}
		})
	}
}

func TestDecompressDeckUnstable(t *testing.T) {
	testCases := []struct {
		desc        string
		input       string
		output      map[string]int
		shouldError bool
	}{
		{
			desc:        "Empty",
			input:       "",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:        "Invalid card index",
			input:       "card|junk||};;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:        "Negative card index",
			input:       "card|junk||-1;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:        "Out of bounds card index",
			input:       "card|junk||3;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output:      map[string]int{},
			shouldError: true,
		},
		{
			desc:  "Simple deck",
			input: "card|junk||0,1,1,0,2,0;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
			output: map[string]int{
				"card1": 3,
				"card2": 2,
				"card3": 1,
			},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			d := &deck{
				buf: []byte(tc.input),
			}

			actualOutput, err := decompressDeck(tc.input)
			if tc.shouldError {
				assert.Equal(t, true, err != nil)

				_, err = d.Bytes()
				assert.Equal(t, true, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, len(actualOutput), len(tc.output))
			for i := range actualOutput {
				assert.Equal(t, actualOutput[i], tc.output[i])
			}

			stableOut := getStableOutput(actualOutput)

			unstableOut, err := d.Bytes()
			assert.NilError(t, err)

			assert.Equal(t, string(unstableOut), stableOut)
		})
	}
}

// getBigDeckString makes us a 52 card compressed deck string for testing with.
func getBigDeckString() string {
	compressionDict := make([]string, 0, 128)

	// Generate cards.
	compressedCards := make([]string, 0, 52)
	for i := 0; i < 52; i++ {
		compressionDict = append(compressionDict, fmt.Sprintf("Card Name %d;other details;junk", i))
		compressedCards = append(compressionDict, fmt.Sprintf("%c", WILDCARDS[i]))
	}
	cardsPart := strings.Join(compressedCards, ";;")

	// Generate deck.
	deckIndices := make([]string, 0, 100)
	for i := 0; i < 52; i++ {
		deckIndices = append(deckIndices, fmt.Sprintf("%d", i))
	}
	for i := 0; i < 48; i++ {
		deckIndices = append(deckIndices, "17")
	}
	deckPart := strings.Join(deckIndices, ",")

	// Put it all together.
	compressionPart := strings.Join(compressionDict, "|")
	compressedPart := fmt.Sprintf("%s;;;%s", deckPart, cardsPart)
	return fmt.Sprintf("%s||%s", compressionPart, compressedPart)
}

func TestDecompressBigDeck(t *testing.T) {
	output, err := decompressDeck(getBigDeckString())
	assert.NilError(t, err)

	assert.Equal(t, len(output), 52)
	for card, count := range output {
		switch card {
		case "Card Name 17":
			assert.Equal(t, count, 49, card)
		default:
			assert.Equal(t, count, 1, card)
		}
	}
}

// old handler code for formatting result
func getStableOutput(countMap map[string]int) string {
	keys := make([]string, 0, len(countMap))
	for k := range countMap {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(i, j string) bool {
		if i == "Ascender's Bane" {
			return false
		}
		if j == "Ascender's Bane" {
			return true
		}
		return i < j
	})

	result := strings.Builder{}
	for _, k := range keys {
		result.WriteString(k)
		result.WriteString(" x")
		result.WriteString(fmt.Sprint(countMap[k]))
		result.WriteString("\n")
	}

	return result.String()
}

func TestDecompressBigDeckUnstable(t *testing.T) {
	deckStr := getBigDeckString()

	res, err := decompressDeck(deckStr)
	assert.NilError(t, err)

	stableOutput := getStableOutput(res)

	deck := &deck{
		buf: []byte(deckStr),
	}

	unstableOut, err := deck.Bytes()
	assert.NilError(t, err)

	assert.Equal(t, string(unstableOut), stableOutput)
}

func BenchmarkDecompressDeck(b *testing.B) {
	testCases := []struct {
		desc  string
		input string
	}{
		{
			desc:  "Simple deck",
			input: "card|junk||0,1,1,0,2,0;;;&01;&1;x;;&02;&1;y;;&03;&1;z",
		},
		{
			desc:  "Big deck",
			input: getBigDeckString(),
		},
	}

	b.ResetTimer()
	for _, tc := range testCases {
		b.Run(tc.desc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				decompressDeck(tc.input)
			}
		})
	}
}

func BenchmarkDecompressDeckUnstable(b *testing.B) {
	testCases := []struct {
		desc  string
		input []byte
	}{
		{
			desc:  "Simple deck",
			input: []byte("card|junk||0,1,1,0,2,0;;;&01;&1;x;;&02;&1;y;;&03;&1;z"),
		},
		{
			desc:  "Big deck",
			input: []byte(getBigDeckString()),
		},
	}

	d := new(deck)
	b.ResetTimer()
	for _, tc := range testCases {
		b.Run(tc.desc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				d.buf = tc.input
				d.parse()
			}
		})
	}
}
