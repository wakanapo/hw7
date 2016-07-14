package othello

import (
	"io/ioutil"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"encoding/json"
	"fmt"
	"net/http"
)

func init() {
	http.HandleFunc("/", getMove)
}

type Game struct {
	Board Board `json:board`
}

func getMove(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	var js []byte
	defer r.Body.Close()
	js, _ = ioutil.ReadAll(r.Body)
	if len(js) < 1 {
		js = []byte(r.FormValue("json"))
	}
	if len(js) < 1 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<body><form method=get>
Paste JSON here:<p/><textarea name=json cols=80 rows=24></textarea>
<p/><input type=submit>
</form>
</body>`)
		return
	}
	var game Game
	err := json.Unmarshal(js, &game)
	if err != nil {
		fmt.Fprintf(w, "invalid json %v? %v", string(js), err)
		return
	}
	board := game.Board
	log.Infof(ctx, "got board: %v", board)
	moves := board.ValidMoves()
	if len(moves) < 1 {
		fmt.Fprintf(w, "PASS")
		return
	}
	move := board.EvaluateFromValidMoves(moves)
	fmt.Fprintf(w, "[%d,%d]", move.Where[0], move.Where[1])
}


type Piece int8

const (
	Empty Piece = iota
	Black Piece = iota
	White Piece = iota

	// Red/Blue are aliases for Black/White
	Red  = Black
	Blue = White
)

func (p Piece) Opposite() Piece {
	switch p {
	case White:
		return Black
	case Black:
		return White
	default:
		return Empty
	}
}

type Board struct {
	// Layout says what pieces are where.
	Pieces [8][8]Piece
	// Next says what the color of the next piece played must be.
	Next Piece
}

// Position represents a position on the othello board. Valid board
// coordinates are 1-8 (not 0-7)!
type Position [2]int

// Valid returns true iff this is a valid board position.
func (p Position) Valid() bool {
	ok := func(i int) bool { return 1 <= i && i <= 8 }
	return ok(p[0]) && ok(p[1])
}

// Pass returns true iff this move position represents a pass.
func (p Position) Pass() bool {
	return !p.Valid()
}

// Move describes a move on an Othello board.
type Move struct {
	// Where a piece is going to be placed. If Where is zeros, or
	// another invalid coordinate, it indicates a pass.
	Where Position
	// As is the player taking the player taking the turn.
	As Piece
}

// At returns a pointer to the piece at a given position.
func (b *Board) At(p Position) *Piece {
	return &b.Pieces[p[1]-1][p[0]-1]
}

// Get returns the piece at a given position.
func (b *Board) Get(p Position) Piece {
	return *b.At(p)
}

// Exec runs a move on a given Board, updating the given board, and
// returning it. Returns error if the move is illegal.
func (b *Board) Exec(m Move) (*Board, error) {
	if !m.Where.Pass() {
		if _, err := b.realMove(m); err != nil {
			return b, err
		}
	} else {
		// Attempting to pass.
		valid := b.ValidMoves()
		if len(valid) > 0 {
			return nil, fmt.Errorf("%v illegal move: there are valid moves available: %v", m, valid)
		}
	}
	b.Next = b.Next.Opposite()
	return b, nil
}

// realMove executes a move that isn't a PASS.
func (b *Board) realMove(m Move) (*Board, error) {
	captures, err := b.tryMove(m)
	if err != nil {
		return nil, err
	}

	for _, p := range append(captures, m.Where) {
		*b.At(p) = m.As
	}
	return b, nil
}

type direction Position

var dirs []direction

func init() {
	for x := -1; x <= 1; x++ {
		for y := -1; y <= 1; y++ {
			if x == 0 && y == 0 {
				continue
			}
			dirs = append(dirs, direction{x, y})
		}
	}
}

// tryMove tries a non-PASS move without actually executing it.
// Returns the list of captures that would happen.
func (b *Board) tryMove(m Move) ([]Position, error) {
	if b.Get(m.Where) != Empty {
		return nil, fmt.Errorf("%v illegal move: %v is occupied by %v", m, m.Where, b.Get(m.Where))
	}

	var captures []Position
	for _, dir := range dirs {
		captures = append(captures, b.findCaptures(m, dir)...)
	}

	if len(captures) < 1 {
		return nil, fmt.Errorf("%v illegal move: no pieces were captured", m)
	}
	return captures, nil
}

func translate(p Position, d direction) Position {
	return Position{p[0] + d[0], p[1] + d[1]}
}

func (b *Board) findCaptures(m Move, dir direction) []Position {
	var caps []Position
	for p := m.Where; true; caps = append(caps, p) {
		p = translate(p, dir)
		if !p.Valid() {
			// End of board.
			return []Position{}
		}
		switch *b.At(p) {
		case m.As:
			return caps
		case Empty:
			return []Position{}
		}
	}
	panic("impossible")
}

func (b *Board) ValidMoves() []Move {
	var moves []Move
	for y := 1; y <= 8; y++ {
		for x := 1; x <= 8; x++ {
			m := Move{Where: Position{x, y}, As: b.Next}
			_, err := b.tryMove(m)
			if err == nil {
				moves = append(moves, m)
			}
		}
	}
	return moves
}

func (b *Board) NextBoard(m Move) Board {
	board := *b
	board.Pieces[m.Where[0] - 1][m.Where[1] - 1] = board.Next
	board.Next = board.Next.Opposite()
	return board
}

func (b *Board) GetGameCount() int {
	cnt := 0
	for y := 1; y <= 8; y++ {
		for x := 1; x <= 8; x++ {
			if b.Pieces[x][y] != 0 {
				cnt += 1
			}
		}
	}
	return cnt
}

func (b *Board) ScoreDifference() int {
	score := 0
	myColor := b.Next
	for y := 1; y <= 8; y++ {
		for x := 1; x <= 8; x++ {
			if b.Pieces[x][y] == myColor {
				score += 1
			}
			if b.Pieces[x][y] == myColor.Opposite() {
				score -= 1
			}
		}
	}
	return score
}

func (b *Board) Evaluate(moves []Move) Move{
	cnt := b.GetGameCount()
	switch {
	case cnt < 30:
		return b.EvaluateFromValidMoves(moves)
	case cnt < 55:
		return b.EvaluateFromBoadStatus(moves)
	default:
		return b.EvaluateFromCaptures(moves)
	}
}

func (b *Board) EvaluateFromBoadStatus(moves []Move) []Move {
	boadEvaluate := [4][4]int{{68, -12, 53, -8},{-12, -62, -33, -7},{53, -33, 26, 8},{-8, -7, 8, -18}}
	max := -100
	var vestMove Move
	for _, move := range moves {
		column := move.Where[0] - 1
		row := move.Where[1] - 1
		if column >= 4 {
			column = 7 - column
		}
		if row >= 4 {
			row = 7 - row
		}
		if boadEvaluate[column][row] > max {
			max = boadEvaluate[column][row]
			vestMove = move
		}
	}
	return vestMove
}

func (b *Board) EvaluateFromValidMoves(moves []Move) Move {
	var vestMove Move
	min := 100
	for _, move := range moves {
		board := *b
		board.realMove(move)
		nextMoves := board.ValidMoves()
		
		if len(nextMoves) < min {
			vestMove = move
			min = len(nextMoves)
		}
	}
	return vestMove
}

func (b *Board) EvaluateFromCaptures(moves []Move) Move {
	var vestMove Move
	max := 0
	for _, move := range moves {
		captures, _ := b.tryMove(move)
		if len(captures) > max {
			vestMove = move
			max = len(captures)
		}
	}
	return vestMove
}
// func (b *Board) Negamax_aux(color Piece, depth int, alpha int, beta int) {
// 	if depth == 0 {
// 		return b.ScoreDifference()
// 	}
// 	moves = b.ValidMoves()
// 	if len(moves) == 0 {
// 		return b.
		
// 	}
// }
