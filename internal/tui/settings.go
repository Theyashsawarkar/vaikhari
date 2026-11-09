package tui

// The Settings tab: LinkedIn's app and sign-in, and how you look on X, all
// here in the screen. It opens read-only: nothing changes until you press
// Enter on a field, so stray typing can't. The secret is never shown, and
// stays as it is when its field is left empty.

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Theyashsawarkar/vaikhari/internal/app"
)

const developersURL = "https://www.linkedin.com/developers/apps/new"

// The fields, in order.
const (
	fieldID = iota
	fieldSecret
	fieldXName
	fieldXHandle
	fieldCount
)

type settingsForm struct {
	in      [fieldCount]textinput.Model
	focus   int
	editing bool   // typing goes into the field at focus
	before  string // that field's value when editing began
}

// newSettings fills the form from the config (never the secret).
func (m *Model) newSettings() *settingsForm {
	c := m.a.Config
	f := &settingsForm{}
	for i := range f.in {
		in := textinput.New()
		in.Prompt = ""
		in.CharLimit = 300
		in.Width = 46
		in.Cursor.SetMode(cursor.CursorStatic)
		f.in[i] = in
	}
	f.in[fieldID].SetValue(c.LinkedIn.ClientID)
	f.in[fieldSecret].EchoMode = textinput.EchoPassword
	f.in[fieldSecret].EchoCharacter = '•'
	f.in[fieldSecret].Placeholder = "paste it from the Auth tab"
	if c.LinkedIn.ClientSecret != "" {
		f.in[fieldSecret].Placeholder = "saved: leave empty to keep it"
	}
	name, handle, _ := m.names()
	f.in[fieldXName].SetValue(c.X.Name)
	f.in[fieldXName].Placeholder = name
	f.in[fieldXHandle].SetValue(strings.TrimPrefix(c.X.Handle, "@"))
	f.in[fieldXHandle].Placeholder = handle
	f.setFocus(fieldID)
	return f
}

func (f *settingsForm) setFocus(i int) { f.focus = (i + fieldCount) % fieldCount }

func (f *settingsForm) startEdit() tea.Cmd {
	f.editing, f.before = true, f.in[f.focus].Value()
	f.in[f.focus].CursorEnd()
	return f.in[f.focus].Focus()
}

func (f *settingsForm) stopEdit(keep bool) {
	if !keep {
		f.in[f.focus].SetValue(f.before)
	}
	f.editing = false
	f.in[f.focus].Blur()
}

// changes is what saving would change: anything, and LinkedIn's app.
func (m *Model) changes() (any, app bool) {
	f, c := m.form, m.a.Config
	id, secret := strings.TrimSpace(f.in[fieldID].Value()), strings.TrimSpace(f.in[fieldSecret].Value())
	app = id != c.LinkedIn.ClientID || secret != "" && secret != c.LinkedIn.ClientSecret
	any = app || strings.TrimSpace(f.in[fieldXName].Value()) != c.X.Name ||
		strings.TrimPrefix(strings.TrimSpace(f.in[fieldXHandle].Value()), "@") != strings.TrimPrefix(c.X.Handle, "@")
	return any, app
}

// goTab switches tabs: Compose comes back in the mode you left it in (in its
// editor after Tab while writing, NORMAL after Esc, so H/L or 1-5 never land
// you typing), Settings on a fresh form.
func (m *Model) goTab(t int) {
	if m.tab == tabCompose && !m.approving {
		m.resume = m.editing
	}
	if m.editing {
		m.ed.Blur()
	}
	if m.tab == tabSettings && m.form != nil {
		if changed, _ := m.changes(); changed {
			m.flash("Your changes in Settings weren't saved (Ctrl+S saves).", true)
		}
	}
	m.editing = false
	m.tab = t
	switch t {
	case tabCompose:
		if m.resume {
			m.editing = true
			m.ed.Focus()
		}
	case tabSettings:
		m.form = m.newSettings()
	}
}

// settingsKey takes the keys Settings has; the rest (Tab, 1-5, q, ?) go on to
// the ones every tab has, unless a field is being edited.
func (m *Model) settingsKey(k tea.KeyMsg) (tea.Cmd, bool) {
	f := m.form
	s := k.String()
	if f.editing {
		switch s {
		case "enter":
			f.stopEdit(true)
		case "esc":
			f.stopEdit(false)
		case "ctrl+s":
			f.stopEdit(true)
			return m.saveSettings(), true
		case "tab", "shift+tab": // the next field, as in any form: nothing is lost
			f.stopEdit(true)
			if s == "tab" {
				f.setFocus(f.focus + 1)
			} else {
				f.setFocus(f.focus - 1)
			}
		default:
			var cmd tea.Cmd
			f.in[f.focus], cmd = f.in[f.focus].Update(k)
			return cmd, true
		}
		return nil, true
	}
	switch s {
	case "esc":
		m.goTab(tabCompose)
	case "up", "k":
		f.setFocus(f.focus - 1)
	case "down", "j":
		f.setFocus(f.focus + 1)
	case "enter": // only this: a letter would let stray typing start an edit
		return f.startEdit(), true
	case "ctrl+s":
		return m.saveSettings(), true
	case "ctrl+l":
		if changed, _ := m.changes(); changed {
			return m.saveSettings(), true
		}
		return m.askSignIn(), true
	case "ctrl+o":
		if err := m.a.Opener(developersURL); err != nil {
			m.flash(err.Error(), true)
		}
	case "ctrl+y":
		if err := copyText(m.a.Config.LinkedInApp().Redirect()); err != nil {
			m.flash(err.Error(), true)
		} else {
			m.flash("The redirect URL is on your clipboard: add it on LinkedIn's Auth tab.", false)
		}
	default:
		return nil, false
	}
	return nil, true
}

// storeSettings writes what's in the form to the config.
func (m *Model) storeSettings() error {
	f := m.form
	c := m.a.Config
	id, secret := strings.TrimSpace(f.in[fieldID].Value()), strings.TrimSpace(f.in[fieldSecret].Value())
	if id != c.LinkedIn.ClientID && secret == "" {
		c.LinkedIn.ClientSecret = "" // a secret belongs to its app
	}
	c.LinkedIn.ClientID = id
	if secret != "" {
		c.LinkedIn.ClientSecret = secret
	}
	c.X.Name = strings.TrimSpace(f.in[fieldXName].Value())
	c.X.Handle = strings.TrimPrefix(strings.TrimSpace(f.in[fieldXHandle].Value()), "@")
	if err := app.SaveConfig(m.a.Paths, c); err != nil {
		return err
	}
	changed := c.LinkedIn != m.a.Config.LinkedIn
	m.a.Config = c
	m.form = m.newSettings()
	if changed {
		m.a.Tokens.Delete() // that sign-in was through the other app
		m.loadLinkedIn()
	}
	return nil
}

// saveSettings saves, then signs in to LinkedIn if that's what's missing.
// Changing the app of a working sign-in ends that sign-in, so it asks first.
func (m *Model) saveSettings() tea.Cmd {
	changed, app := m.changes()
	if !changed && m.liErr == nil {
		m.flash("Nothing to save.", false)
		return nil
	}
	if app && m.liErr == nil {
		m.confirm = &confirm{title: "Change your LinkedIn app?", lines: []string{
			m.st.Text.Render("You're signed in as " + m.li.Name + " through the app saved now."),
			m.st.Yellow.Render("Saving signs you out, and you sign in again through the new one.")},
			yes: func(m *Model) tea.Cmd { return m.doSaveSettings() }}
		return nil
	}
	return m.doSaveSettings()
}

func (m *Model) doSaveSettings() tea.Cmd {
	if err := m.storeSettings(); err != nil {
		m.flash(err.Error(), true)
		return nil
	}
	c := m.a.Config.LinkedIn
	if c.ClientID == "" && c.ClientSecret == "" {
		m.flash("Saved.", false)
		return nil
	}
	if c.ClientID == "" || c.ClientSecret == "" {
		m.flash("Saved. LinkedIn needs both the Client ID and the Client Secret.", true)
		return nil
	}
	if m.liErr == nil {
		m.flash("Saved.", false)
		return nil
	}
	return m.signIn()
}

// askSignIn signs in, asking first when there's a good sign-in already: it
// opens a browser tab for nothing, and one key is easy to press by accident.
func (m *Model) askSignIn() tea.Cmd {
	if m.liErr != nil {
		return m.signIn()
	}
	days := int(m.li.Left(m.a.Now()).Hours() / 24)
	if days < 7 {
		return m.signIn()
	}
	m.confirm = &confirm{title: "Sign in to LinkedIn again?", lines: []string{
		m.st.Text.Render(fmt.Sprintf("You're signed in as %s, with %d days left: there's no need yet.", m.li.Name, days)),
		m.st.Muted.Render("It opens a tab in your browser.")},
		yes: func(m *Model) tea.Cmd { return m.signIn() }}
	return nil
}

// signIn opens LinkedIn's sign-in in the browser and waits for it right
// here. Esc stops waiting. With no app set up yet, Settings is the way.
func (m *Model) signIn() tea.Cmd {
	c := m.a.Config.LinkedIn
	if c.ClientID == "" || c.ClientSecret == "" {
		if m.tab != tabSettings {
			m.goTab(tabSettings)
		}
		m.flash("LinkedIn needs your app first: the steps are here.", true)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.authCancel = cancel
	m.busy = "Sign in to LinkedIn in your browser… (Esc stops waiting)"
	a := m.a
	return func() tea.Msg {
		defer cancel()
		_, err := a.SignInLinkedIn(ctx, func(string) {})
		return authMsg{err}
	}
}

// The verse in Devanagari is a picture (made with Chromium from Noto Sans
// Devanagari): tmux drops Devanagari's vowel signs from text, which misquotes
// it. Where pictures can't be sharp, Roman letters say it correctly instead.
//
//go:embed verse.png
var versePNG []byte

func (m *Model) verseLines(w int) []string {
	if m.pics.kitty {
		path := filepath.Join(m.a.Paths.State, "verse.png")
		if !m.verseOut { // once a run: this is called at every redraw
			m.verseOut = os.MkdirAll(m.a.Paths.State, 0o700) == nil && os.WriteFile(path, versePNG, 0o600) == nil
		}
		if m.verseOut {
			if l := m.pics.lines(path, w, 4); l != nil {
				return l
			}
		}
	}
	return []string{m.st.Peach.Render("catvāri vāk parimitā padāni tāni vidur brāhmaṇā ye manīṣiṇaḥ"),
		m.st.Peach.Render("guhā trīṇi nihitā neṅgayanti turīyaṃ vāco manuṣyā vadanti")}
}

func (m *Model) settingsView() []string {
	st := m.st
	f := m.form
	field := func(i int, label string) string {
		l := st.Sub.Render(fmt.Sprintf("  %-15s", label))
		switch {
		case f.focus == i && f.editing:
			l = st.Peach.Bold(true).Render(fmt.Sprintf("▌ %-15s", label))
		case f.focus == i:
			l = st.Accent.Bold(true).Render(fmt.Sprintf("▌ %-15s", label))
		}
		out := l + f.in[i].View()
		if f.focus == i && !f.editing {
			out += "  " + st.Muted.Render("Enter edits")
		}
		return out
	}
	step := func(n string) string { return st.Accent.Bold(true).Render(n + ". ") }
	product := st.Peach.Render
	redirect := m.a.Config.LinkedInApp().Redirect()
	status := st.Red.Render("✗ not set up yet")
	switch {
	case m.liErr == nil:
		status = st.Green.Render("✓ signed in as "+m.li.Name) + st.Muted.Render(fmt.Sprintf(", %d days left", int(m.li.Left(m.a.Now()).Hours()/24)))
	case m.a.Config.LinkedIn.ClientID != "" && m.a.Config.LinkedIn.ClientSecret != "":
		status = st.Yellow.Render("◐ app saved, not signed in") + st.Muted.Render(": Ctrl+S signs in")
	}
	lines := []string{
		st.Pill("LinkedIn", st.T.Blue) + "  " + status,
		st.Sub.Render("It posts through a free LinkedIn app of your own. Your posts come from your personal profile."),
		"",
		step("1") + st.Sub.Render("Create an app (any name, a LinkedIn Page as its company): ") + st.Key("^O", "open the page"),
		step("2") + st.Sub.Render("On its Products tab add ") + product("Share on LinkedIn") + st.Sub.Render(" and ") + product("Sign In with LinkedIn using OpenID Connect"),
		step("3") + st.Sub.Render("On its Auth tab add the redirect URL ") + st.Sky.Render(redirect) + "  " + st.Key("^Y", "copy"),
		step("4") + st.Sub.Render("Copy the ") + product("Client ID") + st.Sub.Render(" and ") + product("Primary Client Secret") + st.Sub.Render(" from the same tab into here:"),
		"",
		field(fieldID, "Client ID"),
		field(fieldSecret, "Client Secret"),
		"",
		st.Pill("X", st.T.Sky) + "  " + st.Sub.Render("needs no account here: it opens X's own page with your post filled in. These are for the preview."),
		"",
		field(fieldXName, "Your name"),
		field(fieldXHandle, "Your handle"),
		"",
	}
	if changed, _ := m.changes(); changed {
		lines = append(lines, st.Yellow.Render("● Not saved yet: ")+st.Key("^S", "save"))
	} else {
		lines = append(lines, st.Muted.Render("Nothing here changes until you press Enter on a field."))
	}
	if room := m.bodyHeight() - 2 - len(lines); room >= 7 {
		lines = append(lines, "")
		lines = append(lines, m.verseLines(min(m.w-6, 70))...)
		lines = append(lines, st.Muted.Render("Speech has four measures. Three lie hidden and do not stir; the fourth is what people speak."),
			st.Muted.Render("Ṛgveda 1.164.45 · vaikhari "+app.Version))
	}
	return strings.Split(st.Box(m.w, m.bodyHeight(), "Settings", lines, true, ""), "\n")
}

// clean drops control characters from text an agent or a network wrote: an
// escape sequence in it could repaint the screen around what you approve.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f && !(r >= 0x80 && r < 0xa0)) {
			return r
		}
		return -1
	}, s)
}
