package tui

import (
	"context"
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
		"Folds   after “…", "see more on LinkedIn folds here", "p previews each network",
		"y  Post on both", "1  LinkedIn only", "2  X only"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	key(m, "p")
	if s := screen(m); !strings.Contains(s, "as LinkedIn shows it") || !strings.Contains(s, "…see more") {
		t.Fatal(s)
	}
	key(m, "p")
	if s := screen(m); !strings.Contains(s, "as X shows it") || !strings.Contains(s, "1/2") {
		t.Fatal(s)
	}
	key(m, "p")
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
