package tui

import (
	"fmt"
	"strings"
)

// Key is one key of the screen: what you press, what it does, and where
// (empty: everywhere).
type Key struct {
	Keys, Desc, Where string
}

// Keys is every key, in the order the ? sheet lists them. The README's key
// tables and scripts/keybindings (Vayu's keybinding search) come from it:
// after a change run `go test ./internal/tui -update`.
var Keys = []Key{
	{"Ctrl+S", "Post it now (LinkedIn goes out, X opens in your browser; asks first)", "write"},
	{"Ctrl+T", "Schedule it (tomorrow 9am, fri 6pm, in 2h...)", "write"},
	{"Ctrl+D", "Save it as a draft", "write"},
	{"Ctrl+X", "Toggle posting to X", "write"},
	{"Ctrl+L", "Toggle posting to LinkedIn", "write"},
	{"Ctrl+O", "Add an image (a path; Ctrl+O on an empty line removes them)", "write"},
	{"Ctrl+N", "Start a new post (the current one stays as it was saved)", "write"},
	{"Esc", "Leave the editor (the other keys work then)", "write"},
	{"i / Enter", "Edit the post", "compose"},
	{"Tab / Shift+Tab", "Switch to the next / previous tab", ""},
	{"1 – 5", "Switch to Compose, Queue, Drafts, Posted or Settings (not while writing)", ""},
	{"L", "Open LinkedIn's sign-in again (it lasts 60 days; Settings sets it up)", ""},
	{"?", "Show every key", ""},
	{"q", "Quit", ""},
	{"j / k", "Move to the next / previous post", "lists"},
	{"Enter / e", "Edit the post in Compose", "lists"},
	{"p", "Post it now (asks first)", "lists"},
	{"t", "Schedule it, or move it to another time", "lists"},
	{"o", "Open it on X's page to press Post (and confirm it after)", "lists"},
	{"a", "Approve an agent's post", "lists"},
	{"n", "Decline an agent's post", "lists"},
	{"r", "Send a post LinkedIn refused again", "lists"},
	{"c", "Cancel a scheduled or waiting post", "lists"},
	{"d", "Delete the draft (asks first)", "lists"},
	{"y", "Copy the post's link", "lists"},
	{"v", "Open the post's link in the browser", "lists"},
	{"y", "Approve it as asked (posts on every network it names)", "approve"},
	{"1", "Approve it for LinkedIn only (when it asks for both)", "approve"},
	{"2", "Approve it for X only (when it asks for both)", "approve"},
	{"t", "Schedule it: approve it for another time", "approve"},
	{"e", "Edit it first (Ctrl+S approves your version, Esc comes back)", "approve"},
	{"Tab / Shift+Tab", "Switch to the next / previous capsule: the post, each network, each image", "approve"},
	{"o", "Open the image full size", "approve"},
	{"j / k", "Scroll the post when it's longer than its frame", "approve"},
	{"n", "Decline it (nothing is posted, the agent is told)", "approve"},
	{"q / Esc", "Leave it for later (it waits in Queue)", "approve"},
	{"j / k", "Move to the next / previous field", "settings"},
	{"Enter", "Edit the field (Enter finishes, Esc cancels it)", "settings"},
	{"Ctrl+S", "Save the settings (signs in to LinkedIn if it needs that)", "settings"},
	{"Ctrl+L", "Open LinkedIn's sign-in (saves your changes first)", "settings"},
	{"Ctrl+O", "Open LinkedIn's page for creating your app", "settings"},
	{"Ctrl+Y", "Copy the redirect URL for your LinkedIn app", "settings"},
	{"Esc", "Leave Settings for Compose", "settings"},
	{"Enter", "Confirm it's posted on X (paste its link first for a thread's next part)", "x"},
	{"Ctrl+O", "Open X's page again", "x"},
	{"Ctrl+Y", "Copy the image to your clipboard again", "x"},
	{"Ctrl+K", "Cancel X for this post (LinkedIn is unaffected)", "x"},
	{"Esc", "Leave it for later (it waits in Queue)", "x"},
}

// WebKeys are the keys of `vaikhari web`'s page, for the keybinding search.
var WebKeys = []Key{
	{"Ctrl+Enter", "Post it now, or schedule it when it has a time", "web"},
	{"Ctrl+S", "Save it as a draft", "web"},
	{"Esc", "Leave the open window", "web"},
}

// KeybindingsFile is scripts/keybindings, for Vayu's keybinding search.
func KeybindingsFile() string {
	var b strings.Builder
	b.WriteString("# Keys inside the vaikhari screen, for Vayu's keybinding search\n")
	b.WriteString("# (keybind-search.py reads ~/.tmux/plugins/*/scripts). Generated from\n")
	b.WriteString("# internal/tui/keys.go by `go test ./internal/tui -update`; don't edit.\n")
	// marker is kept apart from the rest of the line: the binary holds this
	// format string, and the keybinding search scans plugins' bin/ folders.
	marker := strings.Join([]string{"#", "keybind"}, " ")
	for _, k := range append(append([]Key{}, Keys...), WebKeys...) {
		where := map[string]string{"web": " (vaikhari web)", "write": " (while writing)", "compose": " (Compose tab)", "lists": " (Queue, Drafts, Posted)", "approve": " (approving an agent's post)", "x": " (X popup)", "settings": " (Settings tab)"}[k.Where]
		fmt.Fprintf(&b, "%s: Tmux/vaikhari | %s | %s%s\n", marker, k.Keys, k.Desc, where)
	}
	return b.String()
}
