# Changelog

Newest first. Releases are tags (`vX.Y.Z`) with `Version` in internal/app/app.go.

## v0.4.1 -- 2026-11-04

Fixes from an independent review of 0.4.0, each with a test:

- **A post can't go out twice from Retry**: two retries at once (a double click, or
  the screen and the page) posted to LinkedIn twice. A retry now claims the post first.
- **No key answers an approval popup as it opens**: `Ctrl+S` and `Enter` could, if
  you were typing elsewhere when it appeared. Now every key waits a moment.
- **An agent's note can't repaint the screen**: escape sequences in it, or in a post's
  text in the lists, are dropped.
- **The browser screen keeps what you see**: a scheduled post whose change is refused
  stays scheduled (it became a draft while the page said "Scheduled"); Post now saves
  what you typed last and keeps it if nothing went out; an agent's post can't get
  images it didn't have; leaving edits to a scheduled or failed post asks first.
- **Approving a post with a refused edit changes nothing** (it used to keep the
  networks or time you'd picked).
- **Opening a post from a list keeps what you were writing**, as a draft.
- **The LinkedIn secret is always private**: config.toml is written 0600, even when
  you made it yourself first.
- **A due X post notifies while `vaikhari web` runs**, as it does otherwise.
- **The browser's settings don't overwrite newer ones**, and a new LinkedIn app there
  ends the old sign-in.
- **Images**: two screens in one kitty window no longer share image ids, and no
  temporary picture is left behind.
- **Settings**: `Tab` while editing a field goes to the next field.
- **X**: numbered thread parts always fit in 280; only real x.com and twitter.com
  links count as a post's link.

## v0.4.0 -- 2026-11-03

- A logo and a shloka at the top of the README and the website: every Vayu tool has them now.
- **`vaikhari web`: the same posts in your browser.** Everything the screen
  does, with previews that look like X's and LinkedIn's own: writing, threads,
  images (pick, paste or drop them), the schedule, the Queue, approvals, X's
  one click, settings and LinkedIn's sign-in. It listens on 127.0.0.1 only and
  every call needs a token that only its page holds. Works on a phone-sized
  window too.
- **Settings tab**: set up LinkedIn inside the screen, with the steps beside the
  fields, and your name and handle for X's preview. Signing in stays in the
  screen too, without dropping to a terminal prompt. It's read-only until you
  press `Enter` on a field, and changing the app of a working sign-in asks first.
- **`Tab` no longer gets stuck in the editor**: `Tab` and `Shift+Tab` switch tabs
  while you write, and what you wrote stays.
- **Signing in again asks first** when your sign-in is still good, so a stray key
  opens no browser tab.
- **Capsules on the approval screen**: `Tab` and `Shift+Tab` switch the frame
  between the post, each network's preview and each image (these replace `p`);
  `o` opens the image full size.
- **Images you can see**: previews draw a post's image where it will sit. In
  kitty it's the real image, even inside a tmux popup; other terminals get
  colour half-blocks. WebP images aren't drawn: `o` opens them.
- **X says what happened to your image**: its window says the image is on your
  clipboard, or why it couldn't be copied, and `Ctrl+Y` copies it again.
- A post of nothing but `---` lines now counts as empty, everywhere.
- `ApproveAs` takes a time below 0 to mean "now".

## v0.3.0 -- 2026-11-03

- **The approval screen as designed**, the same layout as narada's and
  sangam's: networks, place in the queue and "nothing posts until you answer"
  on top; one sentence, what you asked and what's at stake; each network's
  facts (who it posts as, its length against the limit, where LinkedIn folds
  it, X's thread, how it goes out, the image) beside the post exactly as it
  goes, with LinkedIn's fold and X's thread breaks marked; the answers as a
  row of keys. Narrow, the facts stack above the post.
- New answers: `1` / `2` approve a post for LinkedIn or X only, `t` approves
  it for another time, `p` previews it as each network shows it.
- After you answer, the popup says what happened (approved, posted where,
  what still waits for your click, or declined) before it closes.

## v0.2.2 -- 2026-11-03

- The approval popup has the same frame as narada's and sangam's: a red rounded
  border titled "vaikhari · approve a post", red because a post is public.

## v0.2.1 -- 2026-11-02

- **A new approval screen**: one sentence says who wants to post, publicly and
  as you, on which networks; then what you asked, and labels for what's at
  stake. The post sits beside each network's preview, and the answer says what
  it does ("Post on both").
- A key pressed as the popup opens (you were typing elsewhere) can't answer it.

## v0.2.0 -- 2026-11-02

- **Renamed from prasar to vaikhari** (वैखरी, speech spoken aloud: the last of
  the four stages of speech, the one the world hears). The command is
  `vaikhari`, the repository github.com/Theyashsawarkar/vaikhari (the old
  address redirects), the MCP server `vaikhari`. The first run moves your
  folders, database and LinkedIn sign-in to the new name; `kosha update` moves
  the install.
- **A clearer approval**: a post waiting for you reads as one sentence ("Claude Code wants to post to X and LinkedIn"), marked public and can't be unsent, with the agent's reason quoted and the post shown as X and LinkedIn will show it, side by side.
- Screenshots in the README and on the website.

## v0.1.1 -- 2026-10-28

- The previews share their column by what they hold: a long X thread gets
  the room it needs (up to two thirds), so its last part is no longer cut
  off while LinkedIn's box stood half empty.

## v0.1.0 -- 2026-10-28

- First release: write once, post to X and LinkedIn from the terminal.
- **The screen**: an editor with live previews of the post as X shows it
  (X's weighted count, threads split at sentence ends or at `---` lines,
  numbered) and as LinkedIn's feed shows it (folded at "…see more"), each
  with a meter; tabs for Compose, Queue, Drafts and Posted; a key sheet
  (`?`); your varna theme's colours, live. Closing it keeps what you wrote
  as a draft.
- **LinkedIn** through its free API with an app of your own
  (`prasar auth linkedin --setup` walks through it): text with real
  hashtags, images, the 60-day sign-in counted down in the corner.
- **X with one click**, free: X's share page opens with the post filled in,
  you press Post and confirm it (pasting its link makes a thread's next part
  a reply); images go to the clipboard to paste.
- **Drafts and a schedule** (`tomorrow 9am`, `fri 6pm`, `in 2h`...), with
  `prasar service install` publishing scheduled posts in the background;
  a post is claimed before it's sent, so it never goes out twice.
- **For agents**: `prasar mcp` (`post_check`, `post_draft`,
  `post_publish`, `post_list`, `post_accounts`); every post an agent
  proposes waits for your approval in a tmux popup (`prasar approve`), where
  you can edit it.
- **CLI**: `prasar post`, `check`, `list`, `approve`, `auth`, `service`,
  `daemon`, `doctor`, `demo` (a fake LinkedIn, nothing posted).
- A TPM plugin (`prefix B`), an installer, release binaries, and a website.
