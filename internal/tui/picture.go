package tui

// Pictures in the screen. In kitty an image is drawn sharp, through its
// graphics protocol's placeholder characters: they're ordinary text to tmux
// and Bubble Tea, so this works inside a tmux popup, where nothing else
// can draw a real image. The image itself goes straight to the tty of tmux's
// client. Every other terminal gets colour half-blocks.

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// placeholder is kitty's character for a cell of an image.
const placeholder = "\U0010EEEE"

// rowMarks say which row of the image a placeholder cell shows (kitty's
// rowcolumn-diacritics); the cells after the first in a row follow on.
var rowMarks = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F, 0x0346, 0x034A, 0x034B, 0x034C,
	0x0350, 0x0351, 0x0352, 0x0357, 0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484, 0x0485, 0x0486, 0x0487, 0x0592,
	0x0593, 0x0594, 0x0595, 0x0597, 0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1,
	0x05A8, 0x05A9, 0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611, 0x0612, 0x0613, 0x0614, 0x0615,
}

// previewImageRows is how tall an image gets inside a network's preview.
const previewImageRows = 17

type imageInfo struct {
	w, h   int
	format string
	ok     bool
}

type pictures struct {
	kitty        bool   // draw through kitty's protocol
	tty          string // where its image data goes
	cellW, cellH int
	ids          map[string]int       // what's been sent to kitty, by image and size
	next         int                  // the id to use next
	half         map[string][]string  // half-block drawings already made
	failed       map[string]bool      // images that can't be read
	sizes        map[string]imageInfo // what's known of each image, not read again at every redraw
}

// newPictures is half-blocks only: graphics needs a real terminal, so a
// program turns them on with detectGraphics.
func newPictures() *pictures {
	return &pictures{cellW: 1, cellH: 2, ids: map[string]int{}, half: map[string][]string{}, failed: map[string]bool{}, sizes: map[string]imageInfo{},
		next: 16 + os.Getpid()%200}
}

// detectGraphics turns on kitty's protocol when this runs in kitty, directly
// or through tmux, on this machine (it reads the files by path, so not over
// SSH).
func (pp *pictures) detectGraphics() {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return
	}
	if os.Getenv("TMUX") == "" {
		if os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "" {
			pp.kitty, pp.tty = true, "/dev/tty"
		}
		return
	}
	out, err := exec.Command("tmux", "display", "-p", "#{client_termname}\t#{client_tty}\t#{client_cell_width}\t#{client_cell_height}").Output()
	if err != nil {
		return
	}
	f := strings.Split(strings.TrimRight(string(out), "\n"), "\t")
	if len(f) < 2 || !strings.Contains(f[0], "kitty") || f[1] == "" {
		return
	}
	pp.kitty, pp.tty = true, f[1]
	if len(f) == 4 {
		w, _ := strconv.Atoi(f[2])
		h, _ := strconv.Atoi(f[3])
		if w > 0 && h > 0 {
			pp.cellW, pp.cellH = w, h
		}
	}
}

// fit is the largest size in cells that keeps an image's shape in w × h.
func (pp *pictures) fit(iw, ih, w, h int) (int, int) {
	cols := w
	rows := (cols*ih*pp.cellW + iw*pp.cellH/2) / (iw * pp.cellH)
	if rows > h {
		rows = h
		cols = (rows*iw*pp.cellH + ih*pp.cellW/2) / (ih * pp.cellW)
	}
	return max(1, cols), max(1, rows)
}

// imageSize is an image's pixels, or false when it can't be read here.
func imageSize(path string) (int, int, string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, "", false
	}
	defer f.Close()
	c, format, err := image.DecodeConfig(f)
	if err != nil || c.Width == 0 || c.Height == 0 {
		return 0, 0, "", false
	}
	return c.Width, c.Height, format, true
}

// lines draws an image in at most w × h cells; nil when it can't be drawn.
func (pp *pictures) lines(path string, w, h int) []string {
	if w < 1 || h < 1 {
		return nil
	}
	info, known := pp.sizes[path]
	if !known {
		info.w, info.h, info.format, info.ok = imageSize(path)
		pp.sizes[path] = info
	}
	if !info.ok {
		return nil
	}
	iw, ih, format := info.w, info.h, info.format
	if pp.kitty {
		cols, rows := pp.fit(iw, ih, w, min(h, len(rowMarks)))
		if id, err := pp.send(path, format, cols, rows); err == nil {
			return placeholderLines(id, cols, rows)
		}
		pp.kitty = false // a terminal that won't take it: half-blocks from now on
	}
	saved := pp.cellW
	pp.cellW, pp.cellH = 1, 2
	cols, rows := pp.fit(iw, ih, w, h)
	pp.cellW = saved
	return pp.halfBlocks(path, cols, rows)
}

func placeholderLines(id, cols, rows int) []string {
	out := make([]string, rows)
	rest := strings.Repeat(placeholder, cols-1)
	for r := range out {
		out[r] = fmt.Sprintf("\x1b[38;5;%dm%s%c%c%s\x1b[39m", id, placeholder, rowMarks[r], rowMarks[0], rest)
	}
	return out
}

// send gives kitty the image at this size, once.
func (pp *pictures) send(path, format string, cols, rows int) (int, error) {
	key := fmt.Sprintf("%s|%d|%d", path, cols, rows)
	if id, ok := pp.ids[key]; ok {
		return id, nil
	}
	medium, file := "f", path
	if format != "png" { // kitty reads only PNG: it deletes this copy itself
		var err error
		if file, err = pngCopy(path); err != nil {
			return 0, err
		}
		medium = "t"
	}
	id := pp.next
	pp.next = 16 + (pp.next-15)%240
	cmd := fmt.Sprintf("\x1b_Ga=T,U=1,q=2,f=100,t=%s,i=%d,c=%d,r=%d;%s\x1b\\", medium, id, cols, rows, base64.StdEncoding.EncodeToString([]byte(file)))
	if err := pp.write(cmd); err != nil {
		return 0, err
	}
	pp.ids[key] = id
	return id, nil
}

func (pp *pictures) write(s string) error {
	t, err := os.OpenFile(pp.tty, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer t.Close()
	_, err = t.WriteString(s)
	return err
}

// pngCopy writes an image as a PNG in a file kitty may delete (it only does
// that for names holding "tty-graphics-protocol").
func pngCopy(path string) (string, error) {
	im, err := decodeImage(path)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "tty-graphics-protocol-*.png")
	if err != nil {
		return "", err
	}
	defer f.Close()
	return f.Name(), png.Encode(f, im)
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	return im, err
}

// halfBlocks draws an image with ▀, two pixels a cell.
func (pp *pictures) halfBlocks(path string, cols, rows int) []string {
	key := fmt.Sprintf("%s|%d|%d", path, cols, rows)
	if l, ok := pp.half[key]; ok {
		return l
	}
	if pp.failed[path] {
		return nil
	}
	im, err := decodeImage(path)
	if err != nil {
		pp.failed[path] = true
		return nil
	}
	b := im.Bounds()
	at := func(x, y int) [3]uint64 { // the average of what falls in this pixel
		var sum [3]uint64
		var n uint64
		for sy := y * b.Dy() / (rows * 2); sy < max((y+1)*b.Dy()/(rows*2), y*b.Dy()/(rows*2)+1); sy++ {
			for sx := x * b.Dx() / cols; sx < max((x+1)*b.Dx()/cols, x*b.Dx()/cols+1); sx++ {
				r, g, bl, _ := im.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
				sum[0], sum[1], sum[2] = sum[0]+uint64(r>>8), sum[1]+uint64(g>>8), sum[2]+uint64(bl>>8)
				n++
			}
		}
		return [3]uint64{sum[0] / n, sum[1] / n, sum[2] / n}
	}
	out := make([]string, rows)
	for r := range out {
		var s strings.Builder
		for c := 0; c < cols; c++ {
			t, u := at(c, r*2), at(c, r*2+1)
			fmt.Fprintf(&s, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", t[0], t[1], t[2], u[0], u[1], u[2])
		}
		out[r] = s.String() + "\x1b[0m"
	}
	pp.half[key] = out
	return out
}

// close takes what was sent back out of kitty's memory.
func (pp *pictures) close() {
	if !pp.kitty {
		return
	}
	for _, id := range pp.ids {
		_ = pp.write(fmt.Sprintf("\x1b_Ga=d,d=I,q=2,i=%d\x1b\\", id))
	}
	pp.ids = map[string]int{}
}

// Close lets the terminal forget the images this screen sent it.
func (m *Model) Close() { m.pics.close() }
