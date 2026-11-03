package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Theyashsawarkar/vaikhari/internal/app"
	"github.com/Theyashsawarkar/vaikhari/internal/fake"
	"github.com/Theyashsawarkar/vaikhari/internal/linkedin"
	"github.com/Theyashsawarkar/vaikhari/internal/post"
)

func setup(t *testing.T) (*Server, *httptest.Server, *fake.LinkedIn, *app.App) {
	t.Setenv("VAIKHARI_NO_KEYRING", "1")
	t.Setenv("TMUX", "")
	f := fake.New()
	t.Cleanup(f.Close)
	t.Setenv("VAIKHARI_LINKEDIN_URL", f.URL)
	h := t.TempDir()
	a, err := app.Open(app.Paths{Config: filepath.Join(h, "c"), Data: filepath.Join(h, "d"), State: filepath.Join(h, "s")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	a.Opener = func(string) error { t.Error("the server opened a browser"); return nil }
	a.Notifier = func(string, string, string) bool { return false }
	a.Tokens.Save(linkedin.Token{Access: "fake-token", Sub: "fake-member", Name: "Asha Demo", Expires: time.Now().Add(40 * 24 * time.Hour).Unix()})
	s := New(a)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	s.host = strings.TrimPrefix(ts.URL, "http://")
	return s, ts, f, a
}

func call(t *testing.T, s *Server, ts *httptest.Server, method, path string, body any) (int, map[string]any) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, ts.URL+path, rd)
	req.Header.Set("X-Vaikhari-Token", s.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestGuards(t *testing.T) {
	s, ts, _, _ := setup(t)
	resp, _ := http.Get(ts.URL + "/api/state")
	if resp.StatusCode != 403 {
		t.Fatal("no token got in:", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/state", nil)
	req.Header.Set("X-Vaikhari-Token", s.token)
	req.Host = "evil.example:80"
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Fatal("another host got in:", resp.StatusCode)
	}
	page, _ := http.Get(ts.URL + "/")
	var b bytes.Buffer
	b.ReadFrom(page.Body)
	if !strings.Contains(b.String(), s.token) || page.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("the page lacks its token or its CSP")
	}
	if code, _ := call(t, s, ts, "GET", "/api/state", nil); code != 200 {
		t.Fatal(code)
	}
}

func TestCheckNeverNull(t *testing.T) {
	s, ts, _, _ := setup(t)
	_, out := call(t, s, ts, "POST", "/api/check", map[string]string{"text": ""})
	if parts, ok := out["x_parts"].([]any); !ok || len(parts) != 0 {
		t.Fatalf("x_parts: %#v", out["x_parts"])
	}
	_, out = call(t, s, ts, "POST", "/api/check", map[string]string{"text": "a\n---\nb #go"})
	if len(out["x_parts"].([]any)) != 2 || out["li_text"] != "a\n\nb #go" {
		t.Fatal(out)
	}
}

func TestPostThenX(t *testing.T) {
	s, ts, f, a := setup(t)
	code, out := call(t, s, ts, "POST", "/api/submit", map[string]any{"text": "Hello from the web", "networks": []string{"x", "linkedin"}})
	if code != 200 || len(f.Posts()) != 1 {
		t.Fatal(code, out, f.Posts())
	}
	p := out["post"].(map[string]any)
	id := int64(p["id"].(float64))
	code, out = call(t, s, ts, "POST", "/api/posts/"+itoa(id)+"/x-open", nil)
	if code != 200 || !strings.HasPrefix(out["url"].(string), "https://x.com/intent/post?text=Hello+from+the+web") {
		t.Fatal(code, out)
	}
	code, out = call(t, s, ts, "POST", "/api/posts/"+itoa(id)+"/x-posted", map[string]string{"link": "not a link"})
	if code != 422 {
		t.Fatal("a bad link was taken", code)
	}
	code, out = call(t, s, ts, "POST", "/api/posts/"+itoa(id)+"/x-posted", map[string]string{"link": "https://x.com/a/status/9"})
	if code != 200 || out["more"] != false {
		t.Fatal(code, out)
	}
	got, _ := a.Store.Get(id)
	if got.Status != post.Posted {
		t.Fatal(got.Status)
	}
}

func TestScheduleAndWhen(t *testing.T) {
	s, ts, f, _ := setup(t)
	_, out := call(t, s, ts, "POST", "/api/when", map[string]string{"text": "in 2h"})
	if out["ok"] != true {
		t.Fatal(out)
	}
	_, out = call(t, s, ts, "POST", "/api/when", map[string]string{"text": "someday"})
	if out["ok"] != false || out["error"] == "" {
		t.Fatal(out)
	}
	code, out := call(t, s, ts, "POST", "/api/submit", map[string]any{"text": "Later", "networks": []string{"linkedin"}, "when": "in 2h"})
	if code != 200 || out["post"].(map[string]any)["status"] != "scheduled" || len(f.Posts()) != 0 {
		t.Fatal(code, out)
	}
}

func TestDraftsAndApproval(t *testing.T) {
	s, ts, f, a := setup(t)
	_, out := call(t, s, ts, "POST", "/api/drafts", map[string]any{"text": "keep me", "networks": []string{"x"}})
	id := int64(out["id"].(float64))
	_, out = call(t, s, ts, "POST", "/api/drafts", map[string]any{"id": id, "text": "keep me, edited", "networks": []string{"x"}})
	if int64(out["id"].(float64)) != id || out["text"] != "keep me, edited" {
		t.Fatal("the draft wasn't updated in place:", out)
	}
	if code, _ := call(t, s, ts, "DELETE", "/api/posts/"+itoa(id), nil); code != 200 {
		t.Fatal(code)
	}

	ag := a.New()
	ag.Text, ag.Origin, ag.Networks = "agent's", post.Agent, []string{"linkedin"}
	ag, _ = a.Submit(t.Context(), ag, false)
	code, out := call(t, s, ts, "POST", "/api/posts/"+itoa(ag.ID)+"/approve", map[string]string{"text": "agent's, edited"})
	if code != 200 || len(f.Posts()) != 1 || f.Posts()[0].Commentary != "agent's, edited" {
		t.Fatal(code, out, f.Posts())
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// What the page shows when approving is what happens: the networks ticked,
// the time in the field, or now when it's cleared.
func TestApproveAsShown(t *testing.T) {
	s, ts, f, a := setup(t)
	queue := func(sendAt int64) int64 {
		p := a.New()
		p.Text, p.Networks, p.Origin, p.SendAt = "from an agent", []string{"x", "linkedin"}, post.Agent, sendAt
		got, err := a.Submit(context.Background(), p, false)
		if err != nil {
			t.Fatal(err)
		}
		return got.ID
	}
	id := queue(0)
	code, _ := call(t, s, ts, "POST", fmt.Sprintf("/api/posts/%d/approve", id), map[string]any{"networks": []string{"linkedin"}, "when": "tomorrow 9am"})
	got, _ := a.Store.Get(id)
	if code != 200 || got.Status != post.Scheduled || got.On(post.X) || len(f.Posts()) != 0 {
		t.Fatalf("scheduled, LinkedIn only: code %d, %+v, sent %d", code, got, len(f.Posts()))
	}
	id = queue(time.Now().Add(48 * time.Hour).UnixMilli())
	code, _ = call(t, s, ts, "POST", fmt.Sprintf("/api/posts/%d/approve", id), map[string]any{"networks": []string{"linkedin"}, "when": ""})
	got, _ = a.Store.Get(id)
	if code != 200 || got.Status != post.Posted || len(f.Posts()) != 1 {
		t.Fatalf("cleared time posts now: code %d, %+v, sent %d", code, got, len(f.Posts()))
	}
}

func TestTimeMustBeAhead(t *testing.T) {
	s, ts, f, a := setup(t)
	past := a.Now().Add(-48*time.Hour).Format("2006-01-02") + " 10:00"
	if code, out := call(t, s, ts, "POST", "/api/when", map[string]any{"text": past}); code != 200 || out["ok"] != false {
		t.Fatalf("a past time reads as fine: %v", out)
	}
	code, _ := call(t, s, ts, "POST", "/api/submit", map[string]any{"text": "hi", "networks": []string{"linkedin"}, "when": past})
	if code != http.StatusUnprocessableEntity || len(f.Posts()) != 0 {
		t.Fatalf("a past time posted it at once: code %d, sent %d", code, len(f.Posts()))
	}
}

func TestOnlyBreaksIsEmpty(t *testing.T) {
	s, ts, _, _ := setup(t)
	if code, _ := call(t, s, ts, "POST", "/api/submit", map[string]any{"text": "---\n---", "networks": []string{"x"}}); code != http.StatusUnprocessableEntity {
		t.Fatalf("a post of only --- went through: %d", code)
	}
}

func TestPostedPostCantGoAgain(t *testing.T) {
	s, ts, f, _ := setup(t)
	_, out := call(t, s, ts, "POST", "/api/submit", map[string]any{"text": "once", "networks": []string{"linkedin"}})
	id := out["post"].(map[string]any)["id"]
	code, _ := call(t, s, ts, "POST", "/api/submit", map[string]any{"id": id, "text": "once", "networks": []string{"linkedin"}})
	if code != http.StatusUnprocessableEntity || len(f.Posts()) != 1 {
		t.Fatalf("posted twice: code %d, sent %d", code, len(f.Posts()))
	}
}

func TestUploadChecksTheBytes(t *testing.T) {
	s, ts, _, _ := setup(t)
	send := func(name string, data []byte) int {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		fw, _ := w.CreateFormFile("image", name)
		fw.Write(data)
		w.Close()
		req, _ := http.NewRequest("POST", ts.URL+"/api/images", &b)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("X-Vaikhari-Token", s.token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := send("notes.png", []byte("just some text")); code != http.StatusUnprocessableEntity {
		t.Errorf("text named .png was taken: %d", code)
	}
	if code := send("real.png", []byte("\x89PNG\r\n\x1a\n0000000000")); code != 200 {
		t.Errorf("a PNG was refused: %d", code)
	}
}
