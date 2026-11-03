package tui

// The approval screen (`vaikhari approve`, the popup an agent's post
// opens): who wants to post what, what's at stake, each network's facts
// beside the post exactly as it goes, and the answers, one key each. The
// same layout as narada's and sangam's approvals; the popup's red frame
// says it's public.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Theyashsawarkar/vaikhari/internal/post"
	"github.com/Theyashsawarkar/vaikhari/internal/ui"
	"github.com/Theyashsawarkar/vaikhari/internal/when"
)

// answered is what happened, shown a moment before the popup closes.
type answered struct {
	title, sub, tiny string
	bad              bool
}

// What the post frame shows: its capsules, cycled with Tab.
const (
	viewExact = iota
	viewLinkedIn
	viewX
	viewImage // then one for each image: viewImage+i
)

func (m *Model) approveKey(s string) tea.Cmd {
	// A popup can open while you're typing elsewhere: what you were
	// typing can't answer it.
	if time.Since(m.shownAt) < armAfter { // every key: Ctrl+S approves too, Enter edits
		return nil
	}
	p := m.editorPost()
	both := p.On(post.X) && p.On(post.LinkedIn)
	switch s {
	case "y", "ctrl+s":
		return m.approve(nil, 0)
	case "1", "2":
		if !both {
			return nil
		}
		if s == "1" {
			return m.approve([]string{post.LinkedIn}, 0)
		}
		return m.approve([]string{post.X}, 0)
	case "t":
		m.ask("When should it go out?", "tomorrow 9am · fri 6pm · in 2h · 3 nov 10:30", "", func(m *Model, v string) tea.Cmd {
			at, err := when.Parse(v, m.a.Now())
			if err != nil {
				m.flash(err.Error(), true)
				return nil
			}
			return m.approve(nil, at.UnixMilli())
		})
	case "n":
		return m.decline()
	case "e", "i", "enter":
		m.editing = true
		return m.ed.Focus()
	case "tab", "shift+tab":
		views := m.views(p)
		step := 1
		if s == "shift+tab" {
			step = len(views) - 1
		}
		i := slices.Index(views, m.view)
		if i < 0 {
			i = 0
		}
		m.view = views[(i+step)%len(views)]
		m.scroll = 0
	case "o":
		if i := m.view - viewImage; i >= 0 && i < len(p.Images) {
			if err := m.a.Opener(p.Images[i]); err != nil {
				m.flash(err.Error(), true)
			}
		} else if len(p.Images) > 0 {
			m.view = viewImage
			m.scroll = 0
		}
	case "j", "down":
		m.scroll++
	case "k", "up":
		m.scroll = max(0, m.scroll-1)
	case "q", "esc":
		return tea.Quit
	case "?":
		m.help = true
	}
	return nil
}

// views is what Tab cycles through: the post exactly, each network's
// preview, then each image.
func (m *Model) views(p post.Post) []int {
	v := []int{viewExact}
	if p.On(post.LinkedIn) {
		v = append(v, viewLinkedIn)
	}
	if p.On(post.X) {
		v = append(v, viewX)
	}
	for i := range p.Images {
		v = append(v, viewImage+i)
	}
	return v
}

// approve says yes: as asked, or only on some networks, or at another time.
func (m *Model) approve(networks []string, sendAt int64) tea.Cmd {
	t := m.ed.Value()
	var edited *string
	if t != m.cur.Text {
		edited = &t
	}
	m.busy = "Approving…"
	a, id := m.a, m.cur.ID
	return func() tea.Msg {
		got, err := a.ApproveAs(context.Background(), id, edited, networks, sendAt, true)
		return publishedMsg{p: got, err: err, action: "approve"}
	}
}

func (m *Model) decline() tea.Cmd {
	if err := m.a.Decline(m.cur.ID); err != nil {
		m.flash(err.Error(), true)
		return nil
	}
	who := firstNonEmpty(m.cur.Requester, "The agent")
	return m.answered(answered{title: "✗ Declined", sub: "Nothing was posted. " + who + " has been told."})
}

// closeAfter is how long the popup shows what happened.
var closeAfter = 1500 * time.Millisecond

// answered shows what happened, then closes the popup.
func (m *Model) answered(r answered) tea.Cmd {
	if r.tiny == "" {
		r.tiny = "closing"
	} else {
		r.tiny += " · closing"
	}
	m.result = &r
	return tea.Tick(closeAfter, func(time.Time) tea.Msg { return tea.QuitMsg{} })
}

// approvedResult is what an approval did, network by network.
func (m *Model) approvedResult(p post.Post, err error) answered {
	if err != nil {
		return answered{title: "✗ It didn't all go out", sub: err.Error(), tiny: "it waits in vaikhari's Queue: r retries", bad: true}
	}
	if p.Status == post.Scheduled {
		return answered{title: "✓ Approved", sub: "It goes out " + when.Say(time.UnixMilli(p.SendAt), m.a.Now()) + ".", tiny: "c in vaikhari's Queue cancels it"}
	}
	var done, wait []string
	for _, n := range p.Networks {
		switch p.State(n) {
		case post.Done:
			done = append(done, post.Name(n))
		case post.Opened, post.Pending:
			wait = append(wait, post.Name(n))
		}
	}
	r := answered{title: "✓ Approved", sub: "Posted on " + strings.Join(done, " and ") + "."}
	switch {
	case len(done) == 0 && len(wait) > 0:
		r.sub = strings.Join(wait, " and ") + " waits for your click."
	case len(wait) > 0:
		r.sub = "Posted on " + strings.Join(done, " and ") + ". " + strings.Join(wait, " and ") + " waits for your click."
	}
	if len(wait) > 0 {
		r.tiny = "o in vaikhari's Queue opens X again"
	}
	return r
}

// position is where this post is among those waiting for an answer.
func (m *Model) position() (int, int) {
	var ids []int64
	for _, p := range m.lists[tabQueue] {
		if p.Status == post.Approval {
			ids = append(ids, p.ID)
		}
	}
	slices.Sort(ids)
	i := slices.Index(ids, m.cur.ID)
	if i < 0 {
		return 1, max(1, len(ids))
	}
	return i + 1, len(ids)
}

// approveScreen is the whole popup, exactly m.h lines.
func (m *Model) approveScreen() string {
	if m.result != nil {
		return m.resultScreen()
	}
	st := m.st
	p := m.editorPost()
	w := max(20, m.w-2) // a column of padding each side
	head := m.approveHead(p, w)
	foot := flow(m.answers(p), w)
	if m.busy != "" {
		foot = []string{st.Accent.Render(m.busy)}
	}
	if m.msg != "" && m.msgBad && time.Since(m.msgAt) < 6*time.Second {
		foot = append(wrapStyled(st.Red.Render("! "+m.msg), w), foot...)
	}
	room := max(4, m.h-len(head)-len(foot)-1)
	lines := append(append([]string{}, head...), m.approveMiddle(p, w, room)...)
	lines = append(lines, "")
	lines = append(lines, foot...)
	// Too small for all of it: the answers and the post win over the top.
	if extra := len(lines) - m.h; extra > 0 {
		lines = lines[min(extra, len(lines)-1):]
	}
	for len(lines) < m.h {
		lines = append(lines[:len(lines)-len(foot)], append([]string{""}, lines[len(lines)-len(foot):]...)...)
	}
	for i, l := range lines {
		lines[i] = ui.Fit(" "+l, m.w)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) resultScreen() string {
	r := m.result
	big := m.st.Green.Bold(true)
	if r.bad || strings.HasPrefix(r.title, "✗") {
		big = m.st.Red.Bold(true)
	}
	w := max(10, m.w-4)
	lines := []string{big.Render(r.title), ""}
	lines = append(lines, wrapStyled(m.st.Text.Render(r.sub), w)...)
	lines = append(lines, "", m.st.Muted.Render(ansi.Truncate(r.tiny, w, "…")))
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n"))
}

// approveHead: which networks, who wants what in one sentence, what you
// asked, what's at stake.
func (m *Model) approveHead(p post.Post, w int) []string {
	st := m.st
	var nets []string
	for _, n := range p.Networks {
		nets = append(nets, post.Name(n))
	}
	i, of := m.position()
	top := st.Pill(strings.Join(nets, " + "), st.T.Blue) + st.Muted.Render(fmt.Sprintf("  %d of %d", i, of))
	if clock := "waits for you · nothing posts until you answer"; lipgloss.Width(top)+len(clock)+2 <= w {
		top = ui.Row(top, st.Muted.Render(clock), w)
	}
	who := firstNonEmpty(p.Requester, "An agent")
	where := "on " + strings.Join(nets, "")
	if len(nets) > 1 {
		where = fmt.Sprintf("on %d networks", len(nets))
	}
	lines := []string{top, ""}
	lines = append(lines, wrapStyled(st.Bold.Render(who+" wants to post ")+st.Red.Bold(true).Render("publicly, as you")+st.Bold.Render(", "+where), w)...)
	if p.Note != "" {
		lines = append(lines, wrapStyled(st.Muted.Render("You asked: ")+st.Sub.Render("“"+clean(p.Note)+"”"), w)...)
	}
	chips := []string{st.Pill("public", st.T.Red)}
	if m.ed.Value() != m.cur.Text {
		chips = append(chips, st.Pill("edited by you: y posts your version", st.T.Yellow))
	}
	if p.SendAt > 0 {
		chips = append(chips, st.Chip("goes "+when.Say(time.UnixMilli(p.SendAt), m.a.Now()), st.T.Blue))
	} else {
		chips = append(chips, st.Chip("now, not scheduled", st.T.Sub))
	}
	lines = append(lines, "")
	lines = append(lines, flow(chips, w)...)
	return append(lines, "")
}

// approveFacts is each network's facts: who it posts as, how long it is
// there, how it goes out.
func (m *Model) approveFacts(p post.Post, w int) []string {
	st := m.st
	var out []string
	kv := func(k, v string) {
		out = append(out, ansi.Truncate(st.Muted.Render(ui.Fit(k, 8))+v, w, "…"))
	}
	head := func(t string) {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, st.Bold.Render(t))
	}
	_, handle, liName := m.names()
	how := "posts on approval"
	if p.SendAt > 0 {
		how = "posts " + when.Say(time.UnixMilli(p.SendAt), m.a.Now())
	}
	if p.On(post.LinkedIn) {
		head("LinkedIn")
		if m.liErr != nil {
			kv("As", st.Red.Render("not signed in")+st.Muted.Render(" (vaikhari auth linkedin)"))
		} else {
			kv("As", st.Text.Render(liName))
		}
		n := post.LinkedInLength(p.Text)
		c := st.Green
		if n > post.LinkedInLimit {
			c = st.Red
		}
		kv("Length", c.Render(fmt.Sprint(n))+st.Muted.Render(fmt.Sprintf(" of %d", post.LinkedInLimit)))
		if shown, folded := linkedInFold(post.LinkedInBody(p.Text)); folded {
			words := strings.Fields(shown)
			tail := strings.Join(words[max(0, len(words)-3):], " ")
			kv("Folds", st.Text.Render("after “…"+tail+"”")+st.Muted.Render(" (see more)"))
		} else {
			kv("Folds", st.Muted.Render("no, all of it shows"))
		}
		kv("How", st.Text.Render(how))
	}
	if p.On(post.X) {
		head("X")
		kv("As", st.Text.Render("@"+handle))
		parts := post.XParts(p.Text)
		if len(parts) > 1 {
			why := "you split it with ---"
			if breaks := post.Breaks(p.Text); len(breaks) < len(parts) {
				longest := 0
				for _, b := range breaks {
					longest = max(longest, post.XLength(b))
				}
				why = fmt.Sprintf("%d is over %d", longest, post.XLimit)
			}
			kv("Length", st.Yellow.Render(fmt.Sprintf("%d-post thread", len(parts)))+st.Muted.Render(" "+why))
		} else {
			kv("Length", st.Green.Render(fmt.Sprint(post.XLength(p.Text)))+st.Muted.Render(fmt.Sprintf(" of %d", post.XLimit)))
		}
		kv("How", st.Yellow.Render("you press Post on X"))
		if len(p.Images) > 0 {
			kv("", st.Muted.Render("the image waits on your clipboard"))
		}
	}
	if len(p.Images) > 0 {
		head(map[bool]string{true: "Images", false: "Image"}[len(p.Images) > 1])
		for i, img := range p.Images {
			size := ""
			if fi, err := os.Stat(img); err == nil {
				size = " " + st.Muted.Render(humanSize(fi.Size()))
			}
			k := ""
			if i == 0 {
				k = "File"
			}
			kv(k, st.Text.Render(filepath.Base(img))+size)
			if iw, ih, _, ok := imageSize(img); ok {
				kv("Size", st.Text.Render(fmt.Sprintf("%d × %d", iw, ih)))
			}
		}
	}
	return out
}

// approveMiddle lays the facts beside the post, or above it when narrow.
func (m *Model) approveMiddle(p post.Post, w, room int) []string {
	facts := m.approveFacts(p, w)
	two := w >= 100
	factW, frameW, frameH := w, w, room
	if two {
		factW = min(44, w/3)
		frameW = w - factW - 2
		facts = m.approveFacts(p, factW)
	} else {
		frameH = max(4, room-min(len(facts), room/2)-1)
		if keep := max(0, room-frameH-1); len(facts) > keep {
			facts = append(facts[:max(0, keep-1)], m.st.Yellow.Render("… more when the window is taller"))
			facts = facts[:keep]
		}
	}
	frame := append([]string{m.capsules(p, frameW)}, m.postFrame(p, frameW, frameH-1)...)
	if !two {
		return append(append(facts, ""), frame...)
	}
	out := make([]string, len(frame))
	for i := range frame {
		l := ""
		if i < len(facts) {
			l = facts[i]
		}
		out[i] = ui.Fit(l, factW) + "  " + frame[i]
	}
	return out
}

// capsules are what the frame can show, the current one lit.
func (m *Model) capsules(p post.Post, w int) string {
	st := m.st
	var row []string
	for _, v := range m.views(p) {
		name := ""
		switch {
		case v == viewExact:
			name = "The post"
		case v == viewLinkedIn:
			name = "LinkedIn"
		case v == viewX:
			name = "X"
		case len(p.Images) > 1:
			name = fmt.Sprintf("▣ Image %d", v-viewImage+1)
		default:
			name = "▣ Image"
		}
		if v == m.view {
			row = append(row, st.Pill(name, st.T.Accent))
		} else {
			row = append(row, st.Chip(name, st.T.Sub))
		}
	}
	line := strings.Join(row, " ")
	if hint := "Tab / Shift+Tab"; lipgloss.Width(line)+len(hint)+2 <= w {
		return ui.Row(line, st.Muted.Render(hint), w)
	}
	return ansi.Truncate(line, w, "…")
}

// postFrame is the post in a frame: exactly as it goes, with LinkedIn's
// fold and X's thread breaks marked, as one network shows it, or an image.
func (m *Model) postFrame(p post.Post, w, h int) []string {
	iw := max(4, w-4)
	var body []string
	title := "the post, exactly"
	inside := max(1, h-2)
	image := m.view - viewImage
	if image >= len(p.Images) { // it had an image, and doesn't now
		m.view, image = viewExact, -1
	}
	switch m.view {
	case viewLinkedIn:
		title, body = "as LinkedIn shows it", unpad(m.linkedInPreview(p, p.Text, iw, 1000))
	case viewX:
		title, body = "as X shows it", unpad(m.xPreview(p, p.Text, iw, 1000))
	case viewExact:
		if m.ed.Value() != m.cur.Text {
			title = "the post, with your edit"
		}
		body = m.exactLines(p, iw)
	default:
		title = fmt.Sprintf("%s · %d of %d", filepath.Base(p.Images[image]), image+1, len(p.Images))
		if body = m.pics.lines(p.Images[image], iw, inside); body == nil {
			body = []string{m.st.Muted.Render("This image can't be drawn here: o opens it.")}
		}
	}
	scroll := min(m.scroll, max(0, len(body)-inside))
	m.scroll = scroll
	shown := body[scroll:min(len(body), scroll+inside)]
	var note []string
	if len(body) > inside {
		note = append(note, fmt.Sprintf("%d–%d of %d · j/k", scroll+1, scroll+len(shown), len(body)))
	}
	if image >= 0 {
		note = append(note, "o opens it full size")
	}
	return frameBox(m.st, shown, title, strings.Join(note, " · "), w, h)
}

// exactLines is the text as written, wrapped, with a line where LinkedIn
// folds it and where X's thread goes on to the next post.
func (m *Model) exactLines(p post.Post, w int) []string {
	st := m.st
	parts := post.Breaks(p.Text)
	if len(parts) == 0 {
		return []string{st.Muted.Render("(empty)")}
	}
	cut := -1 // where LinkedIn folds, in its text (the parts joined)
	if p.On(post.LinkedIn) {
		if shown, folded := linkedInFold(post.LinkedInBody(p.Text)); folded {
			cut = len(shown)
		}
	}
	fold := st.Muted.Render(ui.Fit("─── see more on LinkedIn folds here ───", w))
	text := func(s string) (out []string) {
		for _, l := range ui.Wrap(s, w) {
			out = append(out, st.Text.Render(l))
		}
		return out
	}
	var out []string
	at := 0
	for i, part := range parts {
		if i > 0 {
			if p.On(post.X) {
				out = append(out, st.Sky.Render("─── next post of the X thread ───"))
			} else {
				out = append(out, "")
			}
			at += 2
		}
		for _, line := range strings.Split(part, "\n") {
			end := at + len(line)
			if cut >= at && cut <= end {
				before, after := line[:cut-at], strings.TrimSpace(line[cut-at:])
				if strings.TrimSpace(before) != "" {
					out = append(out, text(before)...)
				}
				out = append(out, fold)
				if after != "" {
					out = append(out, text(after)...)
				}
				cut = -1
			} else {
				out = append(out, text(line)...)
			}
			at = end + 1
		}
	}
	return out
}

// answers are the keys that answer, as labelled buttons.
func (m *Model) answers(p post.Post) []string {
	st := m.st
	button := func(k, what, bg string) string {
		if bg == "" {
			return st.Key(k, "") + " " + st.Text.Render(what)
		}
		return st.Pill(k, bg) + " " + st.Bold.Render(what)
	}
	y := m.postLabel()
	if m.ed.Value() != m.cur.Text {
		y += ", my version"
	}
	if p.SendAt > 0 {
		y = "Approve for " + when.Say(time.UnixMilli(p.SendAt), m.a.Now())
	}
	out := []string{button("y", y, st.T.Red)}
	if p.On(post.X) && p.On(post.LinkedIn) {
		out = append(out, button("1", "LinkedIn only", ""), button("2", "X only", ""))
	}
	t := "Schedule"
	if p.SendAt > 0 {
		t = "Another time"
	}
	out = append(out, button("e", "Edit first", ""), button("t", t, ""), button("n", "Decline", ""), button("q", "Decide later", ""))
	return out
}

// frameBox is a box with a title on its top left and a note on its top
// right.
func frameBox(s ui.Styles, lines []string, title, note string, w, h int) []string {
	b := ui.FG(s.T.Border)
	inner := w - 2
	t := " " + title + " "
	n := ""
	if note != "" {
		n = " " + note + " "
	}
	if lipgloss.Width(t)+lipgloss.Width(n)+3 > inner {
		n = ""
	}
	t = ansi.Truncate(t, max(0, inner-2), "…")
	fill := max(0, inner-1-lipgloss.Width(t)-lipgloss.Width(n)-1)
	top := b.Render("╭─") + s.Muted.Render(t) + b.Render(strings.Repeat("─", fill)) + s.Muted.Render(n) + b.Render("─╮")
	if n == "" {
		top = b.Render("╭─") + s.Muted.Render(t) + b.Render(strings.Repeat("─", max(0, inner-1-lipgloss.Width(t)))+"╮")
	}
	out := []string{top}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, b.Render("│")+" "+ui.Fit(l, inner-2)+" "+b.Render("│"))
	}
	return append(out, b.Render("╰"+strings.Repeat("─", inner)+"╯"))
}

// flow lays items out in rows of at most w cells, three spaces apart.
func flow(items []string, w int) []string {
	var rows []string
	row := ""
	for _, it := range items {
		if row != "" && lipgloss.Width(row)+3+lipgloss.Width(it) > w {
			rows = append(rows, row)
			row = ""
		}
		if row != "" {
			row += "   "
		}
		row += it
	}
	if row != "" {
		rows = append(rows, row)
	}
	return rows
}

// unpad drops the blank rows a preview pads itself with above its meter.
func unpad(l []string) []string {
	if len(l) < 2 {
		return l
	}
	last := l[len(l)-1]
	l = l[:len(l)-1]
	for len(l) > 0 && strings.TrimSpace(ansi.Strip(l[len(l)-1])) == "" {
		l = l[:len(l)-1]
	}
	return append(l, "", last)
}

func wrapStyled(s string, w int) []string { return strings.Split(ansi.Wrap(s, max(1, w), ""), "\n") }

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// humanSize is a file's size the short way: 240 KB, 1.2 MB.
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}
