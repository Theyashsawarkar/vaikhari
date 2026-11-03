package tui

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/Theyashsawarkar/vaikhari/internal/app"
	"github.com/Theyashsawarkar/vaikhari/internal/fake"
	"github.com/Theyashsawarkar/vaikhari/internal/linkedin"
	"github.com/Theyashsawarkar/vaikhari/internal/post"
	"github.com/Theyashsawarkar/vaikhari/internal/ui"
)

func newApp(t *testing.T) (*app.App, *fake.LinkedIn, *[]string) {
	t.Setenv("VAIKHARI_NO_KEYRING", "1")
	t.Setenv("NO_NERD_FONT", "1")
	t.Setenv("TMUX", "")
	closeAfter = time.Millisecond
	f := fake.New()
	t.Cleanup(f.Close)
	t.Setenv("VAIKHARI_LINKEDIN_URL", f.URL)
	h := t.TempDir()
	a, err := app.Open(app.Paths{Config: filepath.Join(h, "c"), Data: filepath.Join(h, "d"), State: filepath.Join(h, "s")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	var opened []string
	a.Opener = func(u string) error { opened = append(opened, u); return nil }
	a.Clipboard = func(string) error { return nil }
	a.Notifier = func(string, string, string) bool { return false }
	a.Tokens.Save(linkedin.Token{Access: "fake-token", Sub: "fake-member", Name: "Asha Demo", Expires: time.Now().Add(40 * 24 * time.Hour).Unix()})
	return a, f, &opened
}

// No test may open the real browser: one that did opened a tab on the user's
// desktop at every run.
func TestMain(m *testing.M) {
	linkedin.Open = func(u string) error { panic("a test tried to open the browser: " + u) }
	os.Exit(m.Run())
}

func screen(m *Model) string { return ansi.Strip(m.View()) }

func size(m *Model) { m.Update(tea.WindowSizeMsg{Width: 150, Height: 40}) }

func typeText(m *Model, s string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

// run feeds a command's message back, like Bubble Tea does.
func run(m *Model, cmd tea.Cmd) {
	for i := 0; cmd != nil && i < 5; i++ {
		msg := cmd()
		if msg == nil {
			return
		}
		if _, ok := msg.(tea.BatchMsg); ok {
			return
		}
		_, cmd = m.Update(msg)
	}
}

func key(m *Model, k string) tea.Cmd {
	var msg tea.KeyMsg
	switch k {
	case "ctrl+s":
		msg = tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+t":
		msg = tea.KeyMsg{Type: tea.KeyCtrlT}
	case "ctrl+x":
		msg = tea.KeyMsg{Type: tea.KeyCtrlX}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	_, cmd := m.Update(msg)
	return cmd
}

func TestComposeShowsBothPreviews(t *testing.T) {
	a, _, _ := newApp(t)
	m := New(a, Options{})
	size(m)
	typeText(m, "one part\n---\nsecond part #go")
	s := screen(m)
	for _, want := range []string{"Write", "one part 1/2", "second part #go 2/2", "thread of 2", "Asha Demo", "@ashademo", "25/3,000", "✓ Asha Demo"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	key(m, "ctrl+x") // X off
	if !strings.Contains(screen(m), "Not posting to X") {
		t.Error("X still on")
	}
}

func TestPostNowFlow(t *testing.T) {
	a, f, opened := newApp(t)
	m := New(a, Options{})
	size(m)
	typeText(m, "Hello both")
	key(m, "ctrl+s")
	if !strings.Contains(screen(m), "Post it now?") || !strings.Contains(screen(m), "goes out at once, as Asha Demo") {
		t.Fatal(screen(m))
	}
	run(m, key(m, "y"))
	if len(f.Posts()) != 1 || len(*opened) != 1 || !strings.Contains((*opened)[0], "text=Hello+both") {
		t.Fatal(f.Posts(), *opened)
	}
	if !strings.Contains(screen(m), "Post it on X") {
		t.Fatal("no X popup:\n" + screen(m))
	}
	key(m, "enter")
	ps, _ := a.Store.List(post.Posted)
	if len(ps) != 1 {
		t.Fatal("not posted", ps)
	}
	if m.ed.Value() != "" {
		t.Fatal("editor kept the posted text")
	}
}

func TestSchedule(t *testing.T) {
	a, f, _ := newApp(t)
	m := New(a, Options{})
	size(m)
	typeText(m, "Later please")
	key(m, "ctrl+t")
	typeText(m, "in 2h")
	run(m, key(m, "enter"))
	ps, _ := a.Store.List(post.Scheduled)
	if len(ps) != 1 || len(f.Posts()) != 0 {
		t.Fatal(ps, f.Posts())
	}
	if !strings.Contains(screen(m), "Scheduled #") {
		t.Fatal(screen(m))
	}
}

// agentPost is an agent's post waiting for approval, on both networks
// unless networks says otherwise.
func agentPost(t *testing.T, a *app.App, text string, networks ...string) post.Post {
	t.Helper()
	p := a.New()
	p.Text, p.Origin, p.Requester, p.Note = text, post.Agent, "Claude Code", "launch post"
	if len(networks) > 0 {
		p.Networks = networks
	}
	p, err := a.Submit(context.Background(), p, false)
	if err != nil || p.Status != post.Approval {
		t.Fatal(p, err)
	}
	return p
}

func approver(a *app.App, p post.Post) *Model {
	m := New(a, Options{Approve: p.ID})
	size(m)
	m.shownAt = time.Time{}
	return m
}

func TestApproveScreen(t *testing.T) {
	a, f, _ := newApp(t)
	p := agentPost(t, a, "From the agent", post.LinkedIn)
	m := New(a, Options{Approve: p.ID})
	size(m)
	s := screen(m)
	for _, want := range []string{"LinkedIn", "1 of 1", "waits for you · nothing posts until you answer",
		"Claude Code wants to post publicly, as you, on LinkedIn", "You asked: “launch post”", "public", "now, not scheduled",
		"As      Asha Demo", "Length  14 of 3000", "Folds   no, all of it shows", "How     posts on approval",
		"the post, exactly", "From the agent", "y  Post on LinkedIn", "e  Edit first", "t  Schedule", "n  Decline"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	for _, not := range []string{"LinkedIn only", "X only", "p previews"} {
		if strings.Contains(s, not) {
			t.Errorf("%q offered for one network:\n%s", not, s)
		}
	}
	if testing.Verbose() {
		t.Log("\n" + s)
	}
	for _, sz := range [][2]int{{200, 60}, {120, 30}, {90, 24}, {60, 18}, {40, 12}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		v := m.View()
		if h := strings.Count(v, "\n") + 1; h != sz[1] {
			t.Errorf("%v: %d lines", sz, h)
		}
		if !strings.Contains(ansi.Strip(v), "Post on LinkedIn") {
			t.Errorf("%v: the answers are cut off:\n%s", sz, ansi.Strip(v))
		}
	}
	size(m)
	// what you were typing as it opened can't answer it
	m.shownAt = time.Now()
	run(m, key(m, "y"))
	if len(f.Posts()) != 0 {
		t.Fatal("y answered as the popup opened")
	}
	m.shownAt = time.Time{}
	run(m, key(m, "y"))
	if len(f.Posts()) != 1 || f.Posts()[0].Commentary != "From the agent" {
		t.Fatal(f.Posts())
	}
	if s := screen(m); !strings.Contains(s, "✓ Approved") || !strings.Contains(s, "Posted on LinkedIn.") || !strings.Contains(s, "closing") {
		t.Fatal(s)
	}
}

func TestApproveBothNetworks(t *testing.T) {
	a, _, _ := newApp(t)
	a.Config.X.Handle = "asha_builds"
	long := strings.Repeat("Totals now round after tax in every region. ", 7)
	m := approver(a, agentPost(t, a, long))
	s := screen(m)
	for _, want := range []string{"on 2 networks", "As      @asha_builds", "2-post thread", "is over 280", "How     you press Post on X",
		"Folds   after “…", "see more on LinkedIn folds here", "Tab / Shift+Tab",
		"y  Post on both", "1  LinkedIn only", "2  X only"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	m.approveKey("tab")
	if s := screen(m); !strings.Contains(s, "as LinkedIn shows it") || !strings.Contains(s, "…see more") {
		t.Fatal(s)
	}
	m.approveKey("tab")
	if s := screen(m); !strings.Contains(s, "as X shows it") || !strings.Contains(s, "1/2") {
		t.Fatal(s)
	}
	m.approveKey("tab")
	if !strings.Contains(screen(m), "the post, exactly") {
		t.Fatal(screen(m))
	}
}

func TestApproveOneNetworkOnly(t *testing.T) {
	a, f, opened := newApp(t)
	p := agentPost(t, a, "Only on LinkedIn, please")
	m := approver(a, p)
	run(m, key(m, "1"))
	got, _ := a.Store.Get(p.ID)
	if len(f.Posts()) != 1 || len(*opened) != 0 || got.On(post.X) || got.Status != post.Posted {
		t.Fatal(f.Posts(), *opened, got)
	}

	p = agentPost(t, a, "Only on X, please")
	m = approver(a, p)
	run(m, key(m, "2"))
	got, _ = a.Store.Get(p.ID)
	if len(f.Posts()) != 1 || len(*opened) != 1 || got.On(post.LinkedIn) {
		t.Fatal(f.Posts(), *opened, got)
	}
	if !strings.Contains(screen(m), "Post it on X") {
		t.Fatal("X's step didn't open:\n" + screen(m))
	}

	// one network asked for: 1 and 2 do nothing
	p = agentPost(t, a, "X alone", post.X)
	m = approver(a, p)
	run(m, key(m, "1"))
	if got, _ := a.Store.Get(p.ID); got.Status != post.Approval {
		t.Fatal(got)
	}
}

func TestApproveSchedule(t *testing.T) {
	a, f, _ := newApp(t)
	p := agentPost(t, a, "Later, please")
	m := approver(a, p)
	key(m, "t")
	typeText(m, "in 2h")
	run(m, key(m, "enter"))
	got, _ := a.Store.Get(p.ID)
	if got.Status != post.Scheduled || got.SendAt < time.Now().Add(110*time.Minute).UnixMilli() || len(f.Posts()) != 0 {
		t.Fatal(got, f.Posts())
	}
	if s := screen(m); !strings.Contains(s, "✓ Approved") || !strings.Contains(s, "It goes out") {
		t.Fatal(s)
	}
}

func TestApproveDecline(t *testing.T) {
	a, f, _ := newApp(t)
	p := agentPost(t, a, "No thanks")
	m := approver(a, p)
	run(m, key(m, "n"))
	got, _ := a.Store.Get(p.ID)
	if got.Status != post.Declined || len(f.Posts()) != 0 {
		t.Fatal(got)
	}
	if s := screen(m); !strings.Contains(s, "✗ Declined") || !strings.Contains(s, "Claude Code has been told") {
		t.Fatal(s)
	}
}

func TestApproveEdit(t *testing.T) {
	a, f, _ := newApp(t)
	p := agentPost(t, a, "Frist draft", post.LinkedIn)
	m := approver(a, p)
	key(m, "e")
	m.ed.SetValue("First draft")
	key(m, "esc")
	s := screen(m)
	if !strings.Contains(s, "edited by you") || !strings.Contains(s, "the post, with your edit") || !strings.Contains(s, "Post on LinkedIn, my version") {
		t.Fatal(s)
	}
	run(m, key(m, "y"))
	if len(f.Posts()) != 1 || f.Posts()[0].Commentary != "First draft" {
		t.Fatal(f.Posts())
	}
}

func TestQuitKeepsTheDraft(t *testing.T) {
	a, _, _ := newApp(t)
	m := New(a, Options{PopupKey: "B"})
	size(m)
	typeText(m, "unfinished thought")
	key(m, "esc")
	cmd := key(m, "B")
	if cmd == nil {
		t.Fatal("the popup key didn't quit")
	}
	ds, _ := a.Store.List(post.Draft)
	if len(ds) != 1 || ds[0].Text != "unfinished thought" {
		t.Fatal(ds)
	}
}

func TestHelpAndLists(t *testing.T) {
	a, _, _ := newApp(t)
	a.SaveDraft(post.Post{Text: "a draft", Networks: []string{post.X}})
	m := New(a, Options{})
	size(m)
	key(m, "esc")
	key(m, "?")
	if !strings.Contains(screen(m), "WHILE WRITING") || !strings.Contains(screen(m), "QUEUE, DRAFTS, POSTED") {
		t.Fatal(screen(m))
	}
	key(m, "x") // any key closes
	key(m, "3")
	if !strings.Contains(screen(m), "Drafts · 1") || !strings.Contains(screen(m), "a draft") {
		t.Fatal(screen(m))
	}
}

func writePNG(t *testing.T, w, h int) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			im.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	path := filepath.Join(t.TempDir(), "shot.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPictureFitKeepsShape(t *testing.T) {
	pp := newPictures()
	if c, r := pp.fit(1600, 900, 80, 100); c != 80 || r != 23 { // 80 wide, half as tall a cell
		t.Errorf("wide: %d × %d", c, r)
	}
	if c, r := pp.fit(1600, 900, 80, 10); c != 36 || r != 10 {
		t.Errorf("short room: %d × %d", c, r)
	}
}

func TestHalfBlocksFillTheirSize(t *testing.T) {
	path := writePNG(t, 64, 36)
	l := newPictures().lines(path, 32, 20)
	if len(l) != 9 {
		t.Fatalf("%d rows", len(l))
	}
	for _, line := range l {
		if w := ansi.StringWidth(line); w != 32 {
			t.Fatalf("a row is %d wide", w)
		}
	}
	if newPictures().lines(filepath.Join(t.TempDir(), "gone.png"), 32, 20) != nil {
		t.Error("a missing image drew something")
	}
}

// kitty's cells must count as one column each, or the frame around them breaks.
func TestPlaceholdersAreOneCellEach(t *testing.T) {
	for _, line := range placeholderLines(40, 30, 3) {
		if w := ansi.StringWidth(line); w != 30 {
			t.Fatalf("a row is %d wide", w)
		}
		if got := ui.Fit(line, 34); ansi.StringWidth(got) != 34 {
			t.Fatalf("fitted to %d", ansi.StringWidth(got))
		}
	}
}

// Tab goes through the post, each network and each image, and back; the
// image is really drawn, and o opens that one.
func TestApproveCapsules(t *testing.T) {
	a, _, _ := newApp(t)
	var opened string
	a.Opener = func(u string) error { opened = u; return nil }
	path, gone := writePNG(t, 160, 90), filepath.Join(t.TempDir(), "gone.png")
	p := a.New()
	p.Text, p.Networks, p.Images, p.Origin, p.Requester = "With pictures", []string{post.LinkedIn, post.X}, []string{path}, post.Agent, "Claude Code"
	p, err := a.Submit(context.Background(), p, false)
	if err != nil {
		t.Fatal(err)
	}
	m := approver(a, p)
	s := screen(m)
	for _, want := range []string{"The post", "LinkedIn", "▣ Image", "Tab / Shift+Tab", "shot.png", "160 × 90"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	if strings.Contains(s, "1 image") {
		t.Errorf("the old chip is back:\n%s", s)
	}
	m.approveKey("o") // not on the image yet: it goes there
	if opened != "" || m.view != viewImage {
		t.Fatalf("o from the post: opened %q, view %d", opened, m.view)
	}
	if v := m.View(); !strings.Contains(v, "▀") || !strings.Contains(ansi.Strip(v), "o opens it full size") {
		t.Fatalf("no image drawn:\n%s", ansi.Strip(v))
	}
	m.approveKey("o")
	if opened != path {
		t.Fatalf("opened %q", opened)
	}
	m.approveKey("tab") // past the last, round to the first
	if m.view != viewExact {
		t.Fatalf("view %d", m.view)
	}
	m.approveKey("shift+tab")
	if m.view != viewImage {
		t.Fatalf("back round: view %d", m.view)
	}
	for _, sz := range [][2]int{{200, 60}, {120, 30}, {90, 24}, {60, 18}, {40, 12}} { // it fits every window
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		v := m.View()
		if h := strings.Count(v, "\n") + 1; h != sz[1] {
			t.Errorf("%v: %d lines", sz, h)
		}
		for _, l := range strings.Split(v, "\n") {
			if w := ansi.StringWidth(l); w > sz[0] {
				t.Errorf("%v: a line is %d wide", sz, w)
			}
		}
	}
	size(m)
	// an image that's gone says so, and doesn't break the screen
	p.Images = []string{gone}
	m.cur = p
	if s := screen(m); !strings.Contains(s, "can't be drawn here") {
		t.Fatalf("a missing image:\n%s", s)
	}
}

func keyT(m *Model, t tea.KeyType) tea.Cmd { return m.key(tea.KeyMsg{Type: t}) }

// Writing doesn't trap you: Tab leaves the editor for the next tab, the text
// stays, and coming back you're writing again.
func TestTabLeavesTheEditor(t *testing.T) {
	a, _, _ := newApp(t)
	m := New(a, Options{})
	size(m)
	if !m.editing {
		t.Fatal("it should open in the editor")
	}
	typeText(m, "half a thought")
	keyT(m, tea.KeyTab)
	if m.tab != tabQueue || m.editing {
		t.Fatalf("tab %d, editing %v", m.tab, m.editing)
	}
	keyT(m, tea.KeyShiftTab)
	if m.tab != tabCompose || !m.editing || m.ed.Value() != "half a thought" {
		t.Fatalf("tab %d, editing %v, text %q", m.tab, m.editing, m.ed.Value())
	}
	keyT(m, tea.KeyShiftTab) // and round the other way, to Settings
	if m.tab != tabSettings || !strings.Contains(screen(m), "Client Secret") {
		t.Fatalf("tab %d:\n%s", m.tab, screen(m))
	}
	keyT(m, tea.KeyTab)
	if m.tab != tabCompose {
		t.Fatalf("tab %d", m.tab)
	}
}

func TestSettingsSetsUpLinkedIn(t *testing.T) {
	a, _, opened := newApp(t)
	a.Tokens.Delete()
	var signIn string
	old := linkedin.Open
	linkedin.Open = func(u string) error { // the browser, without a browser
		signIn = u
		go func() {
			if resp, err := http.Get(u); err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	t.Cleanup(func() { linkedin.Open = old })
	m := New(a, Options{})
	size(m)
	// L with no app goes to where it's set up
	keyT(m, tea.KeyEsc)
	key(m, "L")
	if m.tab != tabSettings || !strings.Contains(screen(m), "not set up yet") {
		t.Fatalf("tab %d:\n%s", m.tab, screen(m))
	}
	keyT(m, tea.KeyCtrlO)
	if len(*opened) != 1 || *opened != nil && (*opened)[0] != developersURL {
		t.Fatalf("opened %v", *opened)
	}
	for _, r := range "stray typing, every letter here" { // read-only until you press Enter
		if strings.ContainsRune("qL12345jk", r) {
			continue // the keys every tab has
		}
		key(m, string(r))
	}
	if m.form.in[fieldID].Value() != "" || m.form.editing {
		t.Fatalf("stray typing went in: %q", m.form.in[fieldID].Value())
	}
	keyT(m, tea.KeyEnter)
	typeText(m, "typo")
	keyT(m, tea.KeyEsc) // cancels this field, and stays in Settings
	if m.form.in[fieldID].Value() != "" || m.tab != tabSettings || m.form.editing {
		t.Fatalf("value %q, tab %d", m.form.in[fieldID].Value(), m.tab)
	}
	keyT(m, tea.KeyEnter)
	typeText(m, "my-id")
	keyT(m, tea.KeyEnter)
	key(m, "j")
	keyT(m, tea.KeyEnter)
	typeText(m, "s3cret-value")
	if strings.Contains(screen(m), "s3cret-value") {
		t.Fatal("the secret shows on screen")
	}
	keyT(m, tea.KeyEnter)
	key(m, "j")
	key(m, "j")
	keyT(m, tea.KeyEnter)
	typeText(m, "@theyash")
	keyT(m, tea.KeyEnter)
	if s := screen(m); !strings.Contains(s, "Not saved yet") {
		t.Fatal(s)
	}
	cmd := keyT(m, tea.KeyCtrlS)
	if a.Config.LinkedIn.ClientID != "my-id" || a.Config.LinkedIn.ClientSecret != "s3cret-value" || a.Config.X.Handle != "theyash" {
		t.Fatalf("%+v", a.Config)
	}
	c, err := app.LoadConfig(a.Paths)
	if err != nil || c.LinkedIn.ClientSecret != "s3cret-value" {
		t.Fatalf("not saved: %+v %v", c, err)
	}
	if !strings.Contains(m.View(), "Sign in to LinkedIn") || cmd == nil {
		t.Fatal("it should be signing in")
	}
	run(m, cmd)
	if signIn == "" || m.busy != "" || m.liErr != nil {
		t.Fatalf("sign-in url %q, busy %q, err %v", signIn, m.busy, m.liErr)
	}
	if s := screen(m); !strings.Contains(s, "signed in as") {
		t.Fatal(s)
	}
	// the secret stays when its field is left empty, and is never put back in it
	m.goTab(tabSettings)
	if m.form.in[fieldSecret].Value() != "" {
		t.Fatal("the form holds the secret")
	}
	m.form.setFocus(fieldXName)
	keyT(m, tea.KeyEnter)
	typeText(m, "Yash")
	run(m, keyT(m, tea.KeyCtrlS))
	if a.Config.LinkedIn.ClientSecret != "s3cret-value" || a.Config.X.Name != "Yash" || m.liErr != nil {
		t.Fatalf("%+v, %v", a.Config, m.liErr)
	}
	// signed in already: L asks, so a stray key opens no tab
	m.goTab(tabQueue)
	before := signIn
	signIn = ""
	if cmd := key(m, "L"); cmd != nil || !strings.Contains(screen(m), "Sign in to LinkedIn again?") {
		t.Fatalf("L while signed in:\n%s", screen(m))
	}
	key(m, "n")
	if signIn != "" {
		t.Fatal("it opened a tab anyway")
	}
	signIn = before
	m.goTab(tabSettings)
	// changing the app of a working sign-in asks first, and n changes nothing
	m.form.setFocus(fieldID)
	keyT(m, tea.KeyEnter)
	typeText(m, "2")
	keyT(m, tea.KeyCtrlS)
	if !strings.Contains(screen(m), "Change your LinkedIn app?") || a.Config.LinkedIn.ClientID != "my-id" {
		t.Fatalf("%s\n%+v", screen(m), a.Config.LinkedIn.ClientID)
	}
	key(m, "n")
	if a.Config.LinkedIn.ClientID != "my-id" || m.liErr != nil {
		t.Fatal(a.Config.LinkedIn.ClientID, m.liErr)
	}
}

func TestSettingsFitsEveryWindow(t *testing.T) {
	a, _, _ := newApp(t)
	m := New(a, Options{})
	m.goTab(tabSettings)
	for _, sz := range [][2]int{{200, 60}, {120, 30}, {90, 24}, {60, 18}, {40, 12}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		v := m.View()
		if h := strings.Count(v, "\n") + 1; h != sz[1] {
			t.Errorf("%v: %d lines", sz, h)
		}
		if sz[0] >= 60 && !strings.Contains(strings.Split(ansi.Strip(v), "\n")[0], "Settings") {
			t.Errorf("%v: the Settings tab is cut off", sz)
		}
		for _, l := range strings.Split(v, "\n") {
			if w := ansi.StringWidth(l); w > sz[0] {
				t.Errorf("%v: a line is %d wide: %q", sz, w, ansi.Strip(l))
			}
		}
	}
}
