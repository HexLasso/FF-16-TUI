package main

import (
	"encoding/csv"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// Last update
const LastUpdate = "26-Aug-2026"

// Block size is 256 bytes
const BlockSize = 256

// Default values for cmd parameters
const DefDictFileName = "dict.csv"
const DefMinGap = 0
const DefMaxGap = 31
const DefThreshold = 5
const TopN = 10

// Operation ranges
const MinGapLo = 0
const MinGapHi = 127
const MaxGapLo = 0
const MaxGapHi = 127
const ThresholdLo = 1
const ThresholdHi = 255
const MinOnes = 0
const MaxOnes = 16
const FileSizeLo = 256
const FileSizeHi = 16 * 1024 * 1024 // 16 MB
const DictColumnCount = 2
const MaxDictRows = 65536

// Hexdump
const CharLen = 1
const LineLen = 16
const PageLen = 256

// Help hint for errors
const HelpHint = "Run \"ff-16-tui\" without parameters for help."

// Worst case array size for varying gaps
// +1 for gap=0
var GapTable [MaxGapHi + 1]int

type PatternInfo struct {
	First     byte  // First byte of the pattern
	Second    byte  // Second byte of the pattern
	Gap       int   // Number of bytes between First and Second
	Hits      int   // Count of matches
	Positions []int // Positions of matches
}

type UpdateState struct {
	Offset      int64
	MinGap      int
	MaxGap      int
	Threshold   int
	MinOnes     int
	MaxOnes     int
	HighlightId int
}

func PanicIfError(e error) {
	if e != nil {
		panic(e)
	}
}

func IsOutOfRange(val int, lo int, hi int) bool {
	if (val < lo) || (val > hi) {
		return true
	}
	return false
}

func Help() {
	fmt.Printf("FF-16-TUI searches for frequent 16-bit patterns in file. Last update: %s\r\n\r\n", LastUpdate)
	fmt.Printf("ff-16-tui [filename] [-d <filename>]\r\n\r\n")
	fmt.Printf("  <filename>      Target file\r\n")
	fmt.Printf("  -d <filename>   Dictionary file    (Default: %s)\r\n\n", DefDictFileName)
}

func ToPrintable(first byte, second byte) string {
	var firstPrintable byte = '.'
	var secondPrintable byte = '.'
	if (first >= 0x20) && (first <= 0x7E) {
		firstPrintable = first
	}
	if (second >= 0x20) && (second <= 0x7E) {
		secondPrintable = second
	}
	return fmt.Sprintf("%c%c", firstPrintable, secondPrintable)
}

func PrintHexDump(offset int64, data []byte, positions []int) {
	lookupTable := make(map[int]bool, len(positions))
	for _, pos := range positions {
		lookupTable[pos] = true
	}

	for i := 0; i < len(data); i += LineLen {
		end := i + LineLen
		if end > len(data) {
			end = len(data)
		}

		hex := ""
		printable := ""

		for j := i; j < end; j++ {
			if lookupTable[j] {
				hex += fmt.Sprintf("\033[30;43m%02X\033[0m ", data[j])
			} else {
				hex += fmt.Sprintf("%02X ", data[j])

			}

			if data[j] >= 0x20 && data[j] <= 0x7E {
				if lookupTable[j] {
					printable += fmt.Sprintf("\033[30;43m%c\033[0m", data[j])
				} else {
					printable += string(data[j])
				}
			} else {
				if lookupTable[j] {
					printable += "\033[30;43m.\033[0m"
				} else {
					printable += "."
				}
			}
		}

		fmt.Printf("│%08x %-48s │%s│\r\n", offset+int64(i), hex, printable)
	}
}

func Update(inFile *os.File, inFileSize int, blockBuf []byte, dictRecords [][]string, dictRecordCount int, state *UpdateState) {
	// Clear the terminal screen
	fmt.Print("\033[2J\033[H")
	fmt.Printf("┌──────────────────────────────── ←→ ↑↓ PgUp PgDn Home End ───────────[%3d%%]┐\r\n", state.Offset*100/int64(inFileSize))

	// Read block of data
	bytesRead, err := inFile.ReadAt(blockBuf, state.Offset)
	PanicIfError(err)

	blockFreqTable := make(map[string]PatternInfo)

	// Build pattern frequency table for the block
	for gapIdx := state.MinGap; gapIdx <= state.MaxGap; gapIdx++ {
		for bufIdx := 0; bufIdx < bytesRead-1-GapTable[gapIdx]; bufIdx++ {

			ones := bits.OnesCount8(blockBuf[bufIdx]) + bits.OnesCount8(blockBuf[bufIdx+GapTable[gapIdx]+1])

			if ones < state.MinOnes || ones > state.MaxOnes {
				continue
			}

			key := fmt.Sprintf("%02x +(%d) %02x \r\n", blockBuf[bufIdx], GapTable[gapIdx], blockBuf[bufIdx+GapTable[gapIdx]+1])
			hits := blockFreqTable[key].Hits + 1
			positions := append(blockFreqTable[key].Positions, bufIdx)
			positions = append(positions, bufIdx+GapTable[gapIdx]+1)
			blockFreqTable[key] = PatternInfo{
				First:     blockBuf[bufIdx],
				Second:    blockBuf[bufIdx+GapTable[gapIdx]+1],
				Gap:       GapTable[gapIdx],
				Hits:      hits,
				Positions: positions}
		}
	}

	// Get the top 10 patterns of the block
	topHits := [TopN]int{}
	topKeys := [TopN]string{}

	for k, v := range blockFreqTable {
		hits := v.Hits

		for i := range TopN {
			// Higher hits first
			// If there are multiple patterns with the same hits, choose deterministically
			if hits > topHits[i] || (hits == topHits[i] && (topKeys[i] == "" || k < topKeys[i])) {
				for j := TopN - 1; j > i; j-- {
					topHits[j] = topHits[j-1]
					topKeys[j] = topKeys[j-1]
				}
				topHits[i] = hits
				topKeys[i] = k
				break
			}
		}
	}

	PrintHexDump(state.Offset, blockBuf, blockFreqTable[topKeys[state.HighlightId]].Positions)

	fmt.Printf("├─────────────────────────────────────────── 0-9 Highlight ─────────────────┤\r\n")

	fmt.Printf("│# Pattern      Ascii Ones Freq Dict                                        │\r\n")
	for i := 0; i < TopN; i++ {
		hex := "-"
		printable := "-"
		hitFreq := "-"
		dict := "-"
		if topHits[i] >= state.Threshold {
			if blockFreqTable[topKeys[i]].Gap == 0 {
				hex = fmt.Sprintf("%02X %02X", blockFreqTable[topKeys[i]].First, blockFreqTable[topKeys[i]].Second)
			} else {
				hex = fmt.Sprintf("%02X +(%d) %02X", blockFreqTable[topKeys[i]].First, blockFreqTable[topKeys[i]].Gap, blockFreqTable[topKeys[i]].Second)
			}

			printable = ToPrintable(blockFreqTable[topKeys[i]].First, blockFreqTable[topKeys[i]].Second)
			printable = "|" + printable + "|"

			hitFreq = strconv.Itoa(blockFreqTable[topKeys[i]].Hits)

			for i := 0; i < dictRecordCount; i++ {
				if strings.EqualFold((dictRecords[i])[0], hex) {
					dict = strings.Trim((dictRecords[i])[1], " ")
				}
			}

			ones := bits.OnesCount8(blockFreqTable[topKeys[i]].First) + bits.OnesCount8(blockFreqTable[topKeys[i]].Second)
			if state.HighlightId == i {
				fmt.Printf("│\033[30;43m%d %-12s %5s   %2d %4s %s\033[0m", i, hex, printable, ones, hitFreq, dict)
			} else {
				fmt.Printf("│%d %-12s %5s   %2d %4s %s", i, hex, printable, ones, hitFreq, dict)
			}

			fmt.Printf("\r\033[77G│\r\n")
		} else {
			fmt.Printf("│                                                                           │\r\n")
		}
	}

	fmt.Printf("├── q/a  w/s ────── e/d  r/f ────────── t/g ────────────────────────────────┤\r\n")
	fmt.Printf("│Gap: %d..%d\r\033[16G│Ones: %d..%d\r\033[30G│Threshold: %d\r\033[77G│\r\n", state.MinGap, state.MaxGap, state.MinOnes, state.MaxOnes, state.Threshold)
	fmt.Printf("└─────────────────────────────────────────────────────────────────── x Exit ┘\r\n")
}

func main() {
	// At least one parameter (i.e. filename) is required
	if len(os.Args) < 2 {
		Help()
		return
	}

	// Parameter parsing
	fileName := ""
	dictFileName := DefDictFileName
	minGap := DefMinGap
	maxGap := DefMaxGap
	threshold := DefThreshold
	minOnes := MinOnes
	maxOnes := MaxOnes
	missingValue := false

	for i := 1; i < len(os.Args); i++ {
		arg := strings.ToLower(os.Args[i])
		if strings.EqualFold(arg, "-d") {
			if len(os.Args) <= i+1 {
				missingValue = true
			} else {
				dictFileName = os.Args[i+1]
				i++
			}

		} else {
			if fileName == "" {
				fileName = os.Args[i]
			} else {
				fmt.Printf("ERROR: Unknown parameter \"%s\".\r\n", os.Args[i])
				fmt.Printf("%s\r\n", HelpHint)
				return
			}
		}

		if missingValue {
			fmt.Printf("ERROR: Missing value for \"%s\".\r\n", os.Args[i])
			fmt.Printf("%s\r\n", HelpHint)
			return
		}
	}

	// Opening the input file
	inFile, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("ERROR: File not found \"%s\".\r\n", fileName)
		return
	}

	fi, err := inFile.Stat()
	PanicIfError(err)
	if fi.IsDir() {
		fmt.Printf("ERROR: The supplied parameter \"%s\" is a directory, but a file was expected.\r\n", fileName)
		return
	}

	inFileSize := int(fi.Size())
	if IsOutOfRange(inFileSize, FileSizeLo, FileSizeHi) {
		fmt.Printf("ERROR: File too large. Maximum allowed size is %d MB.\r\n", FileSizeHi/1024/1024)
		return
	}

	// Reading the dictionary (i.e. csv file)
	dictRecordCount := 0
	var dictRecords [][]string
	csvFileContent, err := os.ReadFile(dictFileName)
	if err == nil {
		r := csv.NewReader(strings.NewReader(string(csvFileContent)))
		r.Comma = ';'
		r.Comment = '#'
		dictRecords, err = r.ReadAll()
		PanicIfError(err)
		dictRecordCount = len(dictRecords)
		// Verification
		if dictRecordCount > MaxDictRows {
			fmt.Printf("ERROR: Dictionary file \"%s\" contains %d rows, but the maximum allowed is %d.\r\n", dictFileName, dictRecordCount, MaxDictRows)
			return
		}
		columnCount := len(dictRecords[0])
		if columnCount != DictColumnCount {
			fmt.Printf("ERROR: Dictionary file \"%s\" contains %d columns, but it must contain %d columns.\r\n", dictFileName, columnCount, DictColumnCount)
			return
		}
	} else {
		fmt.Printf("WARNING: Dictionary file not found \"%s\".\r\n", dictFileName)
	}

	// Init
	blockBuf := make([]byte, BlockSize)
	for i := MinGapLo; i <= MaxGapHi; i++ {
		GapTable[i] = i
	}
	offset := int64(0)

	highlightId := 0
	state := UpdateState{
		Offset:      offset,
		MinGap:      minGap,
		MaxGap:      maxGap,
		Threshold:   threshold,
		MinOnes:     minOnes,
		MaxOnes:     maxOnes,
		HighlightId: highlightId,
	}

	Update(inFile, inFileSize, blockBuf, dictRecords, dictRecordCount, &state)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	PanicIfError(err)

	defer term.Restore(int(os.Stdin.Fd()), oldState)

	for {
		buf := make([]byte, 1)
		os.Stdin.Read(buf)

		switch buf[0] {
		case 'x':
			return
		case 'q':
			state.MinGap += 1
			if state.MinGap > MaxGapHi {
				state.MinGap = MaxGapHi
			}
		case 'a':
			state.MinGap -= 1
			if state.MinGap < DefMinGap {
				state.MinGap = DefMinGap
			}
		case 'w':
			state.MaxGap += 1
			if state.MaxGap > MaxGapHi {
				state.MaxGap = MaxGapHi
			}
		case 's':
			state.MaxGap -= 1
			if state.MaxGap < DefMinGap {
				state.MaxGap = DefMinGap
			}
		case 'e':
			state.MinOnes += 1
			if state.MinOnes > MaxOnes {
				state.MinOnes = MaxOnes
			}
		case 'd':
			state.MinOnes -= 1
			if state.MinOnes < MinOnes {
				state.MinOnes = MinOnes
			}
		case 'r':
			state.MaxOnes += 1
			if state.MaxOnes > MaxOnes {
				state.MaxOnes = MaxOnes
			}
		case 'f':
			state.MaxOnes -= 1
			if state.MaxOnes < MinOnes {
				state.MaxOnes = MinOnes
			}
		case 't':
			state.Threshold += 1
			if state.Threshold > ThresholdHi {
				state.Threshold = ThresholdHi
			}
		case 'g':
			state.Threshold -= 1
			if state.Threshold < ThresholdLo {
				state.Threshold = ThresholdLo
			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			state.HighlightId = int(buf[0] - '0')
		case '\x1b':
			// ESC
			os.Stdin.Read(buf)
			if buf[0] != '[' {
				continue
			}
			// CSI
			os.Stdin.Read(buf)
			switch buf[0] {
			case 'A':
				// Up arrow
				state.Offset = max(state.Offset-LineLen, 0)
			case 'B':
				// Down arrow
				state.Offset = min(state.Offset+LineLen, int64(inFileSize)-BlockSize)
			case 'C':
				// Right arrow
				state.Offset = min(state.Offset+CharLen, int64(inFileSize)-BlockSize)
			case 'D':
				// Left arrow
				state.Offset = max(state.Offset-CharLen, 0)
			case 'H':
				// Home
				state.Offset = 0
			case 'F':
				// End
				state.Offset = int64(inFileSize) - BlockSize
			case '5':
				// Page up
				os.Stdin.Read(buf)
				if buf[0] == '~' {
					// Page up
					state.Offset = max(state.Offset-PageLen, 0)
				}
			case '6':
				// Page down
				os.Stdin.Read(buf)
				if buf[0] == '~' {
					// Page down
					state.Offset = min(state.Offset+PageLen, int64(inFileSize)-BlockSize)
				}
			}
		}

		Update(inFile, inFileSize, blockBuf, dictRecords, dictRecordCount, &state)
	}
}
