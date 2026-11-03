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
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// placeholder is kitty's character for a cell of an image.
const placeholder = "\U0010EEEE"

// rowMarks say which row of the image a placeholder cell shows (kitty's
// rowcolumn-diacritics); the cells after the first in a row follow on. The
// same marks carry the high byte of the image's id.
var rowMarks = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F, 0x0346, 0x034A, 0x034B, 0x034C,
	0x0350, 0x0351, 0x0352, 0x0357, 0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484, 0x0485, 0x0486, 0x0487, 0x0592,
	0x0593, 0x0594, 0x0595, 0x0597, 0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1,
	0x05A8, 0x05A9, 0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611, 0x0612, 0x0613, 0x0614, 0x0615,
	0x0616, 0x0617, 0x0657, 0x0658, 0x0659, 0x065A, 0x065B, 0x065D, 0x065E, 0x06D6, 0x06D7, 0x06D8,
	0x06D9, 0x06DA, 0x06DB, 0x06DC, 0x06DF, 0x06E0, 0x06E1, 0x06E2, 0x06E4, 0x06E7, 0x06E8, 0x06EB,
	0x06EC, 0x0730, 0x0732, 0x0733, 0x0735, 0x0736, 0x073A, 0x073D, 0x073F, 0x0740, 0x0741, 0x0743,
	0x0745, 0x0747, 0x0749, 0x074A, 0x07EB, 0x07EC, 0x07ED, 0x07EE, 0x07EF, 0x07F0, 0x07F1, 0x07F3,
	0x0816, 0x0817, 0x0818, 0x0819, 0x081B, 0x081C, 0x081D, 0x081E, 0x081F, 0x0820, 0x0821, 0x0822,
	0x0823, 0x0825, 0x0826, 0x0827, 0x0829, 0x082A, 0x082B, 0x082C, 0x082D, 0x0951, 0x0953, 0x0954,
	0x0F82, 0x0F83, 0x0F86, 0x0F87, 0x135D, 0x135E, 0x135F, 0x17DD, 0x193A, 0x1A17, 0x1A75, 0x1A76,
	0x1A77, 0x1A78, 0x1A79, 0x1A7A, 0x1A7B, 0x1A7C, 0x1B6B, 0x1B6D, 0x1B6E, 0x1B6F, 0x1B70, 0x1B71,
	0x1B72, 0x1B73, 0x1CD0, 0x1CD1, 0x1CD2, 0x1CDA, 0x1CDB, 0x1CE0, 0x1DC0, 0x1DC1, 0x1DC3, 0x1DC4,
	0x1DC5, 0x1DC6, 0x1DC7, 0x1DC8, 0x1DC9, 0x1DCB, 0x1DCC, 0x1DD1, 0x1DD2, 0x1DD3, 0x1DD4, 0x1DD5,
	0x1DD6, 0x1DD7, 0x1DD8, 0x1DD9, 0x1DDA, 0x1DDB, 0x1DDC, 0x1DDD, 0x1DDE, 0x1DDF, 0x1DE0, 0x1DE1,
	0x1DE2, 0x1DE3, 0x1DE4, 0x1DE5, 0x1DE6, 0x1DFE, 0x20D0, 0x20D1, 0x20D4, 0x20D5, 0x20D6, 0x20D7,
	0x20DB, 0x20DC, 0x20E1, 0x20E7, 0x20E9, 0x20F0, 0x2CEF, 0x2CF0, 0x2CF1, 0x2DE0, 0x2DE1, 0x2DE2,
	0x2DE3, 0x2DE4, 0x2DE5, 0x2DE6, 0x2DE7, 0x2DE8, 0x2DE9, 0x2DEA, 0x2DEB, 0x2DEC, 0x2DED, 0x2DEE,
	0x2DEF, 0x2DF0, 0x2DF1, 0x2DF2, 0x2DF3, 0x2DF4, 0x2DF5, 0x2DF6, 0x2DF7, 0x2DF8, 0x2DF9, 0x2DFA,
	0x2DFB, 0x2DFC, 0x2DFD, 0x2DFE, 0x2DFF, 0xA66F, 0xA67C, 0xA67D, 0xA6F0, 0xA6F1, 0xA8E0, 0xA8E1,
	0xA8E2, 0xA8E3, 0xA8E4, 0xA8E5,
}

// maxRows is the tallest image drawn: rows past this get no picture.
const maxRows = 60

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
	next         int                  // the id to use next: its high byte is this process's own
	half         map[string][]string  // half-block drawings already made
	failed       map[string]bool      // images that can't be read
	sizes        map[string]imageInfo // what's known of each image, not read again at every redraw
}

// newPictures is half-blocks only: graphics needs a real terminal, so a
// program turns them on with detectGraphics.
func newPictures() *pictures {
	return &pictures{cellW: 1, cellH: 2, ids: map[string]int{}, half: map[string][]string{}, failed: map[string]bool{}, sizes: map[string]imageInfo{},
		next: (1+rand.IntN(255))<<8 | 1}
}

// detectGraphics turns on kitty's protocol when this runs in kitty, directly
// or through tmux, on this machine (it reads the files by path, so not over
// SSH).
func (pp *pictures) detectGraphics() {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return
	}
	if os.Getenv("TMUX") == "" {
		// TERM, not KITTY_WINDOW_ID: another terminal started from kitty inherits that
		if os.Getenv("TERM") == "xterm-kitty" {
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
		cols, rows := pp.fit(iw, ih, w, min(h, maxRows))
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

// placeholderLines draws image id: its low byte is the cells' colour, its
// high byte their third mark (kitty's protocol; 256 colours pass through tmux).
func placeholderLines(id, cols, rows int) []string {
	out := make([]string, rows)
	rest := strings.Repeat(placeholder, cols-1)
	for r := range out {
		out[r] = fmt.Sprintf("\x1b[38;5;%dm%s%c%c%c%s\x1b[39m", id&0xff, placeholder, rowMarks[r], rowMarks[0], rowMarks[id>>8], rest)
	}
	return out
}

// kittyID is the id kitty files the image under, from the one drawn.
func kittyID(id int) int { return (id>>8)<<24 | id&0xff }

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
	if pp.next&0xff == 0xff { // the low byte goes 1-255; then the next high byte
		pp.next = (pp.next>>8%255+1)<<8 | 1
	} else {
		pp.next++
	}
	cmd := fmt.Sprintf("\x1b_Ga=T,U=1,q=2,f=100,t=%s,i=%d,c=%d,r=%d;%s\x1b\\", medium, kittyID(id), cols, rows, base64.StdEncoding.EncodeToString([]byte(file)))
	if err := pp.write(cmd); err != nil {
		if medium == "t" {
			os.Remove(file) // kitty never got it, so it won't delete it
		}
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
		_ = pp.write(fmt.Sprintf("\x1b_Ga=d,d=I,q=2,i=%d\x1b\\", kittyID(id)))
	}
	pp.ids = map[string]int{}
}

// Close lets the terminal forget the images this screen sent it.
func (m *Model) Close() { m.pics.close() }
