// Package web is vaikhari in the browser: `vaikhari web` serves one page on
// 127.0.0.1 with the same posts, drafts, schedule and approvals as the
// terminal screen, and previews that look like X's and LinkedIn's own.
//
// Only that page can use it: the server listens on loopback, checks the
// Host header (no DNS rebinding), and every API call carries a token that
// is only in the page it serves (no other website can post for you).
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Theyashsawarkar/vaikhari/internal/app"
	"github.com/Theyashsawarkar/vaikhari/internal/linkedin"
	"github.com/Theyashsawarkar/vaikhari/internal/post"
	"github.com/Theyashsawarkar/vaikhari/internal/ui"
	"github.com/Theyashsawarkar/vaikhari/internal/when"
)

//go:embed static
var static embed.FS

// Server is the web screen.
type Server struct {
	a     *app.App
	token string
	host  string     // 127.0.0.1:port, for the Host check
	cfgMu sync.Mutex // settings and setup change the config one at a time
}

// New makes the server for an open vaikhari.
func New(a *app.App) *Server {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	linkedin.Open = func(string) error { return nil } // the page opens LinkedIn's sign-in
	return &Server{a: a, token: hex.EncodeToString(b)}
}

// Listen opens a loopback port (0: any free one) and returns the page's URL.
func (s *Server) Listen(port int) (net.Listener, string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, "", err
	}
	s.host = ln.Addr().String()
	return ln, "http://" + s.host + "/", nil
}

// Serve runs until ctx ends, delivering scheduled posts meanwhile.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = srv.Close()
				return
			case <-t.C:
				s.a.DeliverDue(ctx)
			}
		}
	}()
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Handler is every route.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	files, _ := fs.Sub(static, "static")
	mux.HandleFunc("GET /{$}", s.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(files))))
	mux.HandleFunc("GET /img/{id}/{n}", s.image)
	mux.HandleFunc("GET /upload/{name}", s.uploaded)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/state", s.state)
	api.HandleFunc("GET /api/theme", s.theme)
	api.HandleFunc("POST /api/check", s.check)
	api.HandleFunc("POST /api/when", s.when)
	api.HandleFunc("POST /api/drafts", s.saveDraft)
	api.HandleFunc("POST /api/submit", s.submit)
	api.HandleFunc("POST /api/images", s.upload)
	api.HandleFunc("POST /api/posts/{id}/{action}", s.action)
	api.HandleFunc("DELETE /api/posts/{id}", s.deleteDraft)
	api.HandleFunc("POST /api/settings", s.settings)
	api.HandleFunc("POST /api/auth/linkedin", s.signIn)
	api.HandleFunc("POST /api/auth/linkedin/setup", s.setup)
	mux.Handle("/api/", s.guard(api))

	return s.hostCheck(mux)
}

// hostCheck refuses requests for any other host name (DNS rebinding).
func (s *Server) hostCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, port, _ := net.SplitHostPort(s.host)
		if s.host != "" && r.Host != s.host && r.Host != "localhost:"+port {
			http.Error(w, "wrong host", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// guard lets an API call through only with the page's token.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.tokenOK(r.Header.Get("X-Vaikhari-Token")) {
			http.Error(w, "no token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) tokenOK(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	b, _ := static.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob: https://media.licdn.com; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
	_, _ = w.Write([]byte(strings.ReplaceAll(string(b), "{{TOKEN}}", s.token)))
}

// ---------------------------------------------------------------- JSON

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func read(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// Part is one post of an X thread.
type Part struct {
	Text string `json:"text"`
	Len  int    `json:"len"`
}

// Measure is a text against both networks.
type Measure struct {
	XParts  []Part `json:"x_parts"`
	XLimit  int    `json:"x_limit"`
	LIText  string `json:"li_text"` // the post as LinkedIn gets it
	LILen   int    `json:"li_len"`
	LILimit int    `json:"li_limit"`
	LIFold  string `json:"li_fold"` // what shows before "…see more" ("" when it all shows)
}

func measure(text string) Measure {
	m := Measure{XParts: []Part{}, XLimit: post.XLimit, LILimit: post.LinkedInLimit, LIText: post.LinkedInBody(text), LILen: post.LinkedInLength(text)}
	for _, p := range post.XParts(text) {
		m.XParts = append(m.XParts, Part{p, post.XLength(p)})
	}
	if r := []rune(m.LIText); len(r) > post.LinkedInFold {
		fold := string(r[:post.LinkedInFold])
		if i := strings.LastIndexAny(fold, " \n"); i > post.LinkedInFold*2/3 {
			fold = fold[:i]
		}
		m.LIFold = strings.TrimRight(fold, " \n")
	}
	return m
}

// View is a post as the page shows it.
type View struct {
	post.Post
	Measure
	Say    string   `json:"say,omitempty"` // when it goes out, in words
	Images []string `json:"image_urls"`
	XNext  int      `json:"x_next"` // the X part to open next (1-based)
}

func (s *Server) view(p post.Post) View {
	v := View{Post: p, Measure: measure(p.Text), XNext: p.Results[post.X].Part + 1, Images: []string{}}
	if v.Post.Images == nil {
		v.Post.Images = []string{}
	}
	if v.Post.Networks == nil {
		v.Post.Networks = []string{}
	}
	if p.SendAt > 0 {
		v.Say = when.Say(time.UnixMilli(p.SendAt), s.a.Now())
	}
	for i := range p.Images {
		v.Images = append(v.Images, fmt.Sprintf("/img/%d/%d?t=%s", p.ID, i, s.token))
	}
	return v
}

// ---------------------------------------------------------------- state

type accounts struct {
	LinkedIn struct {
		OK      bool   `json:"ok"`
		Name    string `json:"name,omitempty"`
		Days    int    `json:"days,omitempty"`
		Error   string `json:"error,omitempty"`
		HasApp  bool   `json:"has_app"`
		Picture string `json:"picture,omitempty"`
	} `json:"linkedin"`
	X struct {
		Name   string `json:"name"`
		Handle string `json:"handle"`
	} `json:"x"`
	Networks []string `json:"networks"`
	Redirect string   `json:"redirect"`
	Version  string   `json:"version"`
}

func (s *Server) accounts() accounts {
	var ac accounts
	c := s.a.Config
	ac.LinkedIn.HasApp = c.LinkedIn.ClientID != "" && c.LinkedIn.ClientSecret != ""
	name := "You"
	if t, err := s.a.LinkedIn(); err != nil {
		ac.LinkedIn.Error = err.Error()
	} else {
		ac.LinkedIn.OK, ac.LinkedIn.Name, ac.LinkedIn.Picture = true, t.Name, t.Picture
		ac.LinkedIn.Days = int(t.Left(s.a.Now()).Hours() / 24)
		name = t.Name
	}
	ac.X.Name = c.X.Name
	if ac.X.Name == "" {
		ac.X.Name = name
	}
	ac.X.Handle = strings.TrimPrefix(c.X.Handle, "@")
	if ac.X.Handle == "" {
		ac.X.Handle = strings.ToLower(strings.ReplaceAll(ac.X.Name, " ", ""))
	}
	ac.Networks = c.Networks
	ac.Redirect = c.LinkedInApp().Redirect()
	ac.Version = app.Version
	return ac
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	ps, err := s.a.Store.List()
	if err != nil {
		fail(w, err)
		return
	}
	views := []View{}
	for _, p := range ps {
		if p.Status == post.Cancelled || p.Status == post.Declined {
			continue
		}
		views = append(views, s.view(p))
	}
	writeJSON(w, map[string]any{"stamp": s.a.Store.Stamp(), "accounts": s.accounts(), "posts": views})
}

func (s *Server) theme(w http.ResponseWriter, _ *http.Request) {
	t := ui.LoadTheme("vaikhari")
	light := false
	if b, err := os.ReadFile(ui.Palette()); err == nil {
		var p struct {
			Variant string `json:"variant"`
		}
		_ = json.Unmarshal(b, &p)
		light = p.Variant == "light"
	}
	writeJSON(w, map[string]any{"colors": t, "light": light})
}

func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if err := read(r, &in); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, measure(in.Text))
}

func (s *Server) when(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if err := read(r, &in); err != nil {
		fail(w, err)
		return
	}
	at, err := s.future(in.Text)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "at": at.UnixMilli(), "say": when.Say(at, s.a.Now()), "full": at.Format("Mon 2 Jan, 15:04")})
}

// future reads a time that is still ahead. Core publishes a post whose time
// has passed at once, which is never what someone scheduling it meant.
func (s *Server) future(text string) (time.Time, error) {
	at, err := when.Parse(text, s.a.Now())
	if err == nil && !at.After(s.a.Now()) {
		err = fmt.Errorf("%s has already passed", at.Format("Mon 2 Jan, 15:04"))
	}
	return at, err
}

// ---------------------------------------------------------------- posts

type postIn struct {
	ID       int64    `json:"id"`
	Text     string   `json:"text"`
	Networks []string `json:"networks"`
	Images   []string `json:"images"`
	When     string   `json:"when"`
}

// fill is the stored post (when ID is set) with what the page sent.
func (s *Server) fill(in postIn) (post.Post, error) {
	p := s.a.New()
	if in.ID != 0 {
		got, err := s.a.Store.Get(in.ID)
		if err != nil {
			return p, err
		}
		if got.Status != post.Draft && got.Status != post.Scheduled && got.Status != post.Failed {
			return p, fmt.Errorf("#%d is %s: it can't be changed now", got.ID, got.Status)
		}
		p = got
	}
	p.Text, p.Networks, p.Images, p.SendAt = in.Text, in.Networks, in.Images, 0
	if strings.TrimSpace(in.When) != "" {
		at, err := s.future(in.When)
		if err != nil {
			return p, err
		}
		p.SendAt = at.UnixMilli()
	}
	return p, nil
}

// ready says why Submit would refuse the post, before anything changes.
func ready(p post.Post) error {
	imgs, err := app.ResolveImages(p.Images)
	if err != nil {
		return err
	}
	p.Images, p.Networks = imgs, post.Normalize(p.Networks)
	if msg := post.Check(p); msg != "" {
		return errors.New(msg)
	}
	return nil
}

// claim takes a scheduled post off the schedule before it's changed, so the
// schedule can't publish it at the same moment.
func (s *Server) claim(p post.Post) error {
	if p.ID == 0 || p.Status != post.Scheduled {
		return nil
	}
	c := p
	c.Status = post.Draft
	ok, err := s.a.Store.Transition(c, post.Scheduled)
	if err == nil && !ok {
		err = fmt.Errorf("#%d has just gone out", p.ID)
	}
	return err
}

func (s *Server) saveDraft(w http.ResponseWriter, r *http.Request) {
	var in postIn
	if err := read(r, &in); err != nil {
		fail(w, err)
		return
	}
	p, err := s.fill(in)
	if err == nil {
		err = s.claim(p)
	}
	if err == nil {
		p, err = s.a.SaveDraft(p)
	}
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, s.view(p))
}

func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	var in postIn
	if err := read(r, &in); err != nil {
		fail(w, err)
		return
	}
	p, err := s.fill(in)
	if err == nil {
		err = ready(p) // before the claim: a refused post stays as it was
	}
	if err == nil {
		err = s.claim(p)
	}
	if err != nil {
		fail(w, err)
		return
	}
	p.Status = post.Draft
	p.Origin = post.You
	got, err := s.a.Submit(r.Context(), p, false)
	if got.ID == 0 && err != nil {
		fail(w, err)
		return
	}
	v := s.view(got)
	out := map[string]any{"post": v}
	if err != nil {
		out["error"] = err.Error()
	}
	writeJSON(w, out)
}

func (s *Server) postID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	id, err := s.postID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var in struct {
		Text     *string  `json:"text"`
		Link     string   `json:"link"`
		When     *string  `json:"when"` // approve: nil as asked, "" now
		Networks []string `json:"networks"`
	}
	if r.ContentLength > 0 {
		if err := read(r, &in); err != nil {
			fail(w, err)
			return
		}
	}
	var p post.Post
	out := map[string]any{}
	switch r.PathValue("action") {
	case "x-open":
		var link string
		p, link, err = s.a.XLink(id)
		out["url"] = link
	case "x-posted":
		if in.Link != "" && post.XStatusID(in.Link) == "" {
			err = errors.New("that isn't a link to a post on X (or leave it empty)")
			break
		}
		var more bool
		p, more, err = s.a.PostedX(id, in.Link)
		out["more"] = more
	case "x-skip":
		p, err = s.a.SkipX(id)
	case "approve":
		var sendAt int64
		if in.When != nil {
			sendAt = -1
			if strings.TrimSpace(*in.When) != "" {
				var at time.Time
				if at, err = s.future(*in.When); err != nil {
					break
				}
				sendAt = at.UnixMilli()
			}
		}
		p, err = s.a.ApproveAs(r.Context(), id, in.Text, in.Networks, sendAt, false)
	case "decline":
		err = s.a.Decline(id)
	case "cancel":
		err = s.a.Cancel(id)
	case "retry":
		p, err = s.a.Retry(r.Context(), id)
	case "reschedule":
		var at time.Time
		if in.When == nil {
			err = errors.New("when should it go out?")
		} else if at, err = s.future(*in.When); err == nil {
			p, err = s.a.Reschedule(id, at)
		}
	default:
		http.NotFound(w, r)
		return
	}
	if p.ID == 0 {
		p, _ = s.a.Store.Get(id)
	}
	if p.ID != 0 {
		out["post"] = s.view(p)
	}
	if err != nil {
		out["error"] = err.Error()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	writeJSON(w, out)
}

func (s *Server) deleteDraft(w http.ResponseWriter, r *http.Request) {
	id, err := s.postID(r)
	if err != nil {
		fail(w, err)
		return
	}
	p, err := s.a.Store.Get(id)
	if err == nil && p.Status != post.Draft {
		err = fmt.Errorf("#%d isn't a draft", id)
	}
	if err == nil {
		err = s.a.Store.Delete(id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------- images

func (s *Server) imageDir() string { return filepath.Join(s.a.Paths.Data, "images") }

var imageTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}

// upload keeps an image the page sends in vaikhari's images folder.
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	f, hdr, err := r.FormFile("image")
	if err != nil {
		fail(w, fmt.Errorf("the image: %w (at most 20 MB)", err))
		return
	}
	defer f.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	if _, ok := imageTypes[ext]; !ok || !isImage(head[:n]) {
		fail(w, errors.New("use a PNG, JPEG, GIF or WebP image"))
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fail(w, err)
		return
	}
	if err := os.MkdirAll(s.imageDir(), 0o700); err != nil {
		fail(w, err)
		return
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	name := hex.EncodeToString(b) + ext
	path := filepath.Join(s.imageDir(), name)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		fail(w, err)
		return
	}
	_, err = io.Copy(out, f)
	out.Close()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]string{"path": path, "url": "/upload/" + name + "?t=" + s.token, "name": hdr.Filename})
}

// isImage looks at the bytes: a name can say anything.
func isImage(head []byte) bool {
	for _, ct := range imageTypes {
		if http.DetectContentType(head) == ct {
			return true
		}
	}
	return false
}

// image serves a post's image n (only images a post has).
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	if !s.tokenOK(r.URL.Query().Get("t")) {
		http.Error(w, "no token", http.StatusForbidden)
		return
	}
	id, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	n, err2 := strconv.Atoi(r.PathValue("n"))
	p, err3 := s.a.Store.Get(id)
	if err1 != nil || err2 != nil || err3 != nil || n < 0 || n >= len(p.Images) {
		http.NotFound(w, r)
		return
	}
	s.sendImage(w, r, p.Images[n])
}

// uploaded serves an image from vaikhari's images folder (before it's saved
// in a post).
func (s *Server) uploaded(w http.ResponseWriter, r *http.Request) {
	if !s.tokenOK(r.URL.Query().Get("t")) {
		http.Error(w, "no token", http.StatusForbidden)
		return
	}
	name := r.PathValue("name")
	if name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	s.sendImage(w, r, filepath.Join(s.imageDir(), name))
}

func (s *Server) sendImage(w http.ResponseWriter, r *http.Request, path string) {
	ct, ok := imageTypes[strings.ToLower(filepath.Ext(path))]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", ct)
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// ---------------------------------------------------------------- settings and sign-in

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		XName    string   `json:"x_name"`
		XHandle  string   `json:"x_handle"`
		Networks []string `json:"networks"`
	}
	if err := read(r, &in); err != nil {
		fail(w, err)
		return
	}
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	c, err := app.LoadConfig(s.a.Paths) // the screen may have saved since this started
	if err != nil {
		fail(w, err)
		return
	}
	c.X.Name, c.X.Handle = strings.TrimSpace(in.XName), strings.TrimPrefix(strings.TrimSpace(in.XHandle), "@")
	if ns := post.Normalize(in.Networks); len(ns) > 0 {
		c.Networks = ns
	}
	if err := app.SaveConfig(s.a.Paths, c); err != nil {
		fail(w, err)
		return
	}
	s.a.Config = c
	writeJSON(w, s.accounts())
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID     string `json:"client_id"`
		Secret string `json:"client_secret"`
	}
	if err := read(r, &in); err != nil {
		fail(w, err)
		return
	}
	in.ID, in.Secret = strings.TrimSpace(in.ID), strings.TrimSpace(in.Secret)
	if in.ID == "" || in.Secret == "" {
		fail(w, errors.New("both the Client ID and the Client Secret are needed"))
		return
	}
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	c, err := app.LoadConfig(s.a.Paths) // the screen may have saved since this started
	if err != nil {
		fail(w, err)
		return
	}
	changed := c.LinkedIn.ClientID != in.ID || c.LinkedIn.ClientSecret != in.Secret
	c.LinkedIn.ClientID, c.LinkedIn.ClientSecret = in.ID, in.Secret
	if err := app.SaveConfig(s.a.Paths, c); err != nil {
		fail(w, err)
		return
	}
	s.a.Config = c
	if changed {
		s.a.Tokens.Delete() // that sign-in was through the other app
	}
	writeJSON(w, s.accounts())
}

// signIn starts LinkedIn's sign-in and returns the page to open; the
// state shows the result when the browser comes back.
func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	urls := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_, err := s.a.SignInLinkedIn(ctx, func(u string) { urls <- u })
		errs <- err
	}()
	select {
	case u := <-urls:
		writeJSON(w, map[string]string{"url": u})
	case err := <-errs:
		fail(w, err)
	case <-time.After(5 * time.Second):
		fail(w, errors.New("LinkedIn's sign-in didn't start"))
	}
}
