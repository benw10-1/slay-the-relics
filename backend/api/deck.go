package api

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"
)

const WILDCARDS = "0123456789abcdefghijklmnopqrstvwxyzABCDEFGHIJKLMNOPQRSTVWXYZ_`[]/^%?@><=-+*:;,.()#$!'{}~"

var compressionWildcardRegex []*regexp.Regexp

func init() {
	escapeRegex := regexp.MustCompile(`[-\/\\^$*+?.()|[\]{}]`)

	compressionWildcardRegex = make([]*regexp.Regexp, 0, len(WILDCARDS))
	for i := range WILDCARDS {
		wildCard := fmt.Sprintf("&%c", WILDCARDS[i])
		escaped := escapeRegex.ReplaceAllString(wildCard, "\\$&")
		compressionWildcardRegex = append(compressionWildcardRegex, regexp.MustCompile(escaped))
	}
}

func (a *API) getDeckHandler(c *gin.Context) {
	name := c.Param("name")
	name = strings.ToLower(name)

	deck, ok := func() (*deck, bool) {
		a.deckLock.RLock()
		defer a.deckLock.RUnlock()
		deck, ok := a.deckLists[name]
		return deck, ok
	}()

	if !ok {
		c.JSON(404, gin.H{"error": "deck not found"})
		return
	}

	resBts, err := deck.Bytes()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Data(200, "text/plain", resBts)
}

// deck designed to be parsed once and then used for lookups. The load of the parsing is in the request context as to
// not block the sub
type deck struct {
	// raw data not parsed, after parsing, raw data buf is freed and replaced by ready-to-use result
	buf []byte

	// ensures work is only done once even when racing for deck parse. Once its loaded, will be checked
	// using atomic.LoadUint32 instead of a mutex lock
	parseOnce sync.Once
}

// Bytes use to get parsed deck
func (d *deck) Bytes() (res []byte, err error) {
	d.parseOnce.Do(func() {
		err = d.parse() // if this fails once, it will fail every time
	})
	return d.buf, err
}

// parse use to decompress and get readable str representation of deck, will be called once per uncompressed deck
func (d *deck) parse() error {
	decompressed, err := decompressBytes(d.buf)
	if err != nil {
		return err
	}

	parts := bytes.Split(decompressed, []byte(";;;"))

	// read cards first so we can reference them as we read the card indexes
	cards := make([][]byte, 0, bytes.Count(parts[1], []byte(";;"))+1)
	longestCard := 0

	err = readDelimitedBytes(parts[1], []byte(";;"), func(val []byte) error {
		// TODO: some validation here

		cards = append(cards, val)
		if len(val) > longestCard {
			longestCard = len(val)
		}

		return nil
	})
	if err != nil {
		return err
	}

	cardIdxCountMap := make(map[string]int, len(cards)) // map[card]count
	uniqueCardList := make([]string, 0, len(cards))     // just keep copies of our strs

	// now that we know which cards exist, we can read the indexes
	err = readDelimitedBytes(parts[0], []byte(","), func(val []byte) error {
		// compiler will optimize this string conversion out, see `cloneString` in atoi.go
		cardIdxVal, err := strconv.ParseInt(string(val), 10, 64)
		if err != nil {
			return err
		}

		if cardIdxVal < 0 || cardIdxVal >= int64(len(cards)) {
			return errors.New("card index out of bounds")
		}

		card := parseCardBytes(cards[cardIdxVal])
		fmt.Printf("%d - %s - %s\n", cardIdxVal, string(card), string(cards[cardIdxVal]))

		if len(card) == 0 {
			return nil
		}

		// no guarentee of not copying here in map lookup, so unsafe it out. Is valid as long as `decompressed` is still valid and static
		cardStr := unsafe.String(&card[0], len(card))

		ct, ok := cardIdxCountMap[cardStr]
		if !ok {
			uniqueCardList = append(uniqueCardList, cardStr)
		}
		cardIdxCountMap[cardStr] = ct + 1

		return nil
	})
	if err != nil {
		return err
	}

	// may allocate some extra space in some cases, but we will shrink after we are done formatting
	d.buf = make([]byte, 0, len(cardIdxCountMap)*longestCard)

	// put ascender's bane first
	slices.SortFunc(uniqueCardList, func(i, j string) bool {
		if i == "Ascender's Bane" {
			return false
		}
		if j == "Ascender's Bane" {
			return true
		}

		return i < j
	})

	for _, card := range uniqueCardList {
		// will just use underlying bytes and not do cast
		d.buf = append(d.buf, []byte(card)...)

		cardCount, ok := cardIdxCountMap[card]
		if !ok {
			return errors.New("card not found")
		}

		// fmt - "$card x$count\n"
		if cardCount > 0 {
			d.buf = append(d.buf, ' ', 'x')
			d.buf = strconv.AppendInt(d.buf, int64(cardCount), 10)
			d.buf = append(d.buf, '\n')
		}
	}

	if len(d.buf) < cap(d.buf) {
		d.buf = d.buf[:len(d.buf)]
	}

	return nil
}

func decompressBytes(s []byte) ([]byte, error) {
	parts := bytes.Split(s, []byte("||"))
	if len(parts) < 2 {
		return nil, errors.New("invalid deck")
	}

	compressionDict := bytes.Split(parts[0], []byte("|"))
	text := parts[1]

	for i := len(compressionDict) - 1; i >= 0; i-- {
		word := compressionDict[i]
		// TODO: this is the source of lots of allocs and CPU cycles, probably no need for regexp here
		text = compressionWildcardRegex[i].ReplaceAll(text, word)
	}

	return text, nil
}

type delimCB func(val []byte) error

func readDelimitedBytes(s []byte, delim []byte, cb delimCB) (err error) {
	if len(s) == 0 || (len(s) == 1 && s[0] == '-') {
		return nil
	}

	currIndex := 0
	for currIndex < len(s) {
		nextIndex := currIndex + bytes.Index(s[currIndex:], delim)
		if nextIndex < currIndex {
			nextIndex = len(s)
		}
		err = cb(s[currIndex:nextIndex])
		if err != nil {
			return err
		}

		currIndex = nextIndex + len(delim)
	}

	return nil
}

// parseCardBytes is a helper function to parse the card name from a given section
func parseCardBytes(cardSection []byte) []byte {
	// return first item in triplet
	sectionEnd := bytes.Index(cardSection, []byte(";"))
	if sectionEnd == -1 {
		return cardSection
	}

	return cardSection[:sectionEnd]
}

func decompress(s string) (string, error) {
	parts := strings.Split(s, "||")
	if len(parts) < 2 {
		return "", errors.New("invalid deck")
	}

	compressionDict := strings.Split(parts[0], "|")
	text := parts[1]

	for i := len(compressionDict) - 1; i >= 0; i-- {
		word := compressionDict[i]
		text = compressionWildcardRegex[i].ReplaceAllString(text, word)
	}
	return text, nil
}

func parseCommaDelimitedIntegerArray(s string) ([]int, error) {
	if s == "-" {
		return nil, nil
	}

	currIndex := 0
	result := make([]int, 0, strings.Count(s, ",")+1)
	for currIndex < len(s) {
		nextIndex := currIndex + strings.Index(s[currIndex:], ",")
		if nextIndex < currIndex {
			nextIndex = len(s)
		}

		resultVal, err := strconv.ParseInt(s[currIndex:nextIndex], 10, 64)
		if err != nil {
			return nil, err
		}

		currIndex = nextIndex + 1
		result = append(result, int(resultVal))
	}
	return result, nil
}

func splitDoubleSemicolonArray(s string) []string {
	if s == "-" {
		return nil
	}

	return strings.Split(s, ";;")
}

func parseCards(cardSections []string) []string {
	result := make([]string, 0, len(cardSections))
	for _, cardSection := range cardSections {
		result = append(result, parseCard(cardSection))
	}
	return result
}

func parseCard(cardSection string) string {
	sectionEnd := strings.Index(cardSection, ";")
	if sectionEnd == -1 {
		return cardSection
	}

	return cardSection[:sectionEnd]
}

func decompressDeck(deck string) (map[string]int, error) {
	deck, err := decompress(deck)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(deck, ";;;")
	d, err := parseCommaDelimitedIntegerArray(parts[0])
	if err != nil {
		return nil, err
	}
	cards := parseCards(splitDoubleSemicolonArray(parts[1]))

	deckDict := make(map[string]int, len(cards))
	for _, idx := range d {
		if idx < 0 || idx >= len(cards) {
			return nil, errors.New("card index out of bounds")
		}

		name := cards[idx]
		deckDict[name]++
	}

	return deckDict, nil
}
