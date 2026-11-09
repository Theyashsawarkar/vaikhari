package tui

import (
	"github.com/Theyashsawarkar/vyakarana/keymap"
	"github.com/Theyashsawarkar/vyakarana/mode"
)

// Where keys work: the Compose tab, the lists (Queue, Drafts, Posted),
// Settings, an agent's post waiting for approval, X's step.
const (
	inCompose  = "compose"
	inLists    = "lists"
	inSettings = "settings"
	inApprove  = "approve"
	inX        = "x"
	inWeb      = "web"
)

var writing = []mode.Mode{mode.Insert}

// Keys is every key of the screen, in the order the help sheet lists
// them, with the vim grammar every Vayu tool shares (vyakarana): NORMAL
// keys, a Space leader, INSERT while writing. The README's key tables and
// scripts/keybindings (Vayu's keybinding search) come from it: after a
// change run `go test ./internal/tui -update`. The editor, the approve
// screen, Settings' form and X's step handle their own keys; their rows
// here document them (no action).
var Keys = keymap.New("vaikhari",
	[]keymap.Binding{
		{Seq: "L", Alt: []string{"Tab"}, Act: keymap.ActNextTab, Desc: "Switch to the next tab"},
		{Seq: "H", Alt: []string{"Shift+Tab"}, Act: keymap.ActPrevTab, Desc: "Switch to the previous tab"},
		{Seq: "Space t c", Alt: []string{"1"}, Act: "tab-1", Desc: "Switch to Compose"},
		{Seq: "Space t q", Alt: []string{"2"}, Act: "tab-2", Desc: "Switch to Queue"},
		{Seq: "Space t d", Alt: []string{"3"}, Act: "tab-3", Desc: "Switch to Drafts"},
		{Seq: "Space t p", Alt: []string{"4"}, Act: "tab-4", Desc: "Switch to Posted"},
		{Seq: "Space t s", Alt: []string{"5", "Space , s"}, Act: "tab-5", Desc: "Switch to Settings"},
		{Seq: "Space , l", Act: "sign-in", Desc: "Open LinkedIn's sign-in again (it lasts 60 days; Settings sets it up)"},
		{Seq: "r", Alt: []string{"Space r"}, Act: keymap.ActRefresh, Desc: "Reload the lists"},
		{Seq: "?", Alt: []string{"Space ?"}, Act: keymap.ActHelp, Desc: "Show every key"},
		{Seq: "Space Space", Act: keymap.ActPalette, Desc: "Search every action and run one (the palette)"},
		{Seq: "q", Alt: []string{"Space q"}, Act: keymap.ActQuit, Desc: "Quit (what you were writing stays, as a draft)"},

		{Seq: "i", Alt: []string{"a", "Enter"}, Act: "write", Desc: "Edit the post (INSERT: type it)", Where: inCompose},
		{Seq: "Space p p", Alt: []string{"Ctrl+s"}, Act: "post", Desc: "Post it now (LinkedIn goes out, X opens in your browser; asks first)", Where: inCompose},
		{Seq: "Space p t", Alt: []string{"Ctrl+t"}, Act: "schedule", Desc: "Schedule it (tomorrow 9am, fri 6pm, in 2h...)", Where: inCompose},
		{Seq: "Space p d", Alt: []string{"Ctrl+d"}, Act: "draft", Desc: "Save it as a draft", Where: inCompose},
		{Seq: "Space n x", Alt: []string{"Ctrl+x"}, Act: "toggle-x", Desc: "Toggle posting to X", Where: inCompose},
		{Seq: "Space n l", Alt: []string{"Ctrl+l"}, Act: "toggle-linkedin", Desc: "Toggle posting to LinkedIn", Where: inCompose},
		{Seq: "Space a i", Alt: []string{"Ctrl+o"}, Act: "image", Desc: "Add an image (a path; an empty one removes them)", Where: inCompose},
		{Seq: "Space a n", Alt: []string{"Ctrl+n"}, Act: "new", Desc: "Start a new post (the current one stays as it was saved)", Where: inCompose},

		{Seq: "Ctrl+s", Desc: "Post it now (asks first)", Where: inCompose, Modes: writing},
		{Seq: "Ctrl+t", Desc: "Schedule it", Where: inCompose, Modes: writing},
		{Seq: "Ctrl+d", Desc: "Save it as a draft", Where: inCompose, Modes: writing},
		{Seq: "Ctrl+x", Alt: []string{"Ctrl+l"}, Desc: "Toggle posting to X / LinkedIn", Where: inCompose, Modes: writing},
		{Seq: "Ctrl+o", Desc: "Add an image", Where: inCompose, Modes: writing},
		{Seq: "Ctrl+n", Desc: "Start a new post", Where: inCompose, Modes: writing},
		{Seq: "Tab", Alt: []string{"Shift+Tab"}, Desc: "Switch tabs (what you wrote stays)", Where: inCompose, Modes: writing},
		{Seq: "Esc", Desc: "Leave the editor (NORMAL: the other keys work then)", Where: inCompose, Modes: writing},

		{Seq: "j", Alt: []string{"Down"}, Act: keymap.ActDown, Desc: "Move to the next post", Where: inLists, Count: true},
		{Seq: "k", Alt: []string{"Up"}, Act: keymap.ActUp, Desc: "Move to the previous post", Where: inLists, Count: true},
		{Seq: "g g", Alt: []string{"Home"}, Act: keymap.ActTop, Desc: "Jump to the first post", Where: inLists},
		{Seq: "G", Alt: []string{"End"}, Act: keymap.ActBottom, Desc: "Jump to the last post", Where: inLists},
		{Seq: "Enter", Alt: []string{"e", "Space e e"}, Act: "edit", Desc: "Edit the post in Compose (Posted: open its link)", Where: inLists},
		{Seq: "o", Alt: []string{"Space o b"}, Act: "open-link", Desc: "Open the post's link in the browser", Where: inLists},
		{Seq: "y", Alt: []string{"Space y y"}, Act: "copy-link", Desc: "Copy the post's link", Where: inLists},
		{Seq: "d d", Alt: []string{"Space d d"}, Act: "delete", Desc: "Delete the draft, or cancel a scheduled or waiting post (asks first)", Where: inLists},
		{Seq: "Space p p", Act: "post", Desc: "Post it now (asks first)", Where: inLists},
		{Seq: "Space p t", Act: "schedule", Desc: "Schedule it, or move it to another time", Where: inLists},
		{Seq: "Space p r", Act: "retry", Desc: "Send a post LinkedIn refused again", Where: inLists},
		{Seq: "Space p x", Act: "open-x", Desc: "Open it on X's page to press Post (and confirm it after)", Where: inLists},
		{Seq: "a", Act: "approve", Desc: "Approve an agent's post (asks first)", Where: inLists},
		{Seq: "n", Act: "decline", Desc: "Decline an agent's post", Where: inLists},
		{Seq: "p", Moved: "Space p p", Where: inLists},
		{Seq: "t", Moved: "Space p t", Where: inLists},
		{Seq: "c", Moved: "d d", Where: inLists},
		{Seq: "v", Moved: "o", Where: inLists},

		{Seq: "y", Alt: []string{"Ctrl+s"}, Desc: "Approve it as asked (posts on every network it names)", Where: inApprove},
		{Seq: "1", Desc: "Approve it for LinkedIn only (when it asks for both)", Where: inApprove},
		{Seq: "2", Desc: "Approve it for X only (when it asks for both)", Where: inApprove},
		{Seq: "t", Desc: "Schedule it: approve it for another time", Where: inApprove},
		{Seq: "e", Alt: []string{"i", "Enter"}, Desc: "Edit it first (Ctrl+s approves your version, Esc comes back)", Where: inApprove},
		{Seq: "Tab", Alt: []string{"Shift+Tab"}, Desc: "Switch to the next / previous capsule: the post, each network, each image", Where: inApprove},
		{Seq: "o", Desc: "Open the image full size", Where: inApprove},
		{Seq: "j", Alt: []string{"k"}, Desc: "Scroll the post when it's longer than its frame", Where: inApprove},
		{Seq: "n", Desc: "Decline it (nothing is posted, the agent is told)", Where: inApprove},
		{Seq: "q", Alt: []string{"Esc"}, Desc: "Leave it for later (it waits in Queue)", Where: inApprove},

		{Seq: "j", Alt: []string{"k"}, Desc: "Move to the next / previous field", Where: inSettings},
		{Seq: "Enter", Desc: "Edit the field (Enter finishes, Esc cancels it)", Where: inSettings},
		{Seq: "Ctrl+s", Desc: "Save the settings (signs in to LinkedIn if it needs that)", Where: inSettings},
		{Seq: "Ctrl+l", Desc: "Open LinkedIn's sign-in (saves your changes first)", Where: inSettings},
		{Seq: "Ctrl+o", Desc: "Open LinkedIn's page for creating your app", Where: inSettings},
		{Seq: "Ctrl+y", Desc: "Copy the redirect URL for your LinkedIn app", Where: inSettings},
		{Seq: "Esc", Desc: "Leave Settings for Compose", Where: inSettings},

		{Seq: "Enter", Desc: "Confirm it's posted on X (paste its link first for a thread's next part)", Where: inX, Modes: writing},
		{Seq: "Ctrl+o", Desc: "Open X's page again", Where: inX, Modes: writing},
		{Seq: "Ctrl+y", Desc: "Copy the image to your clipboard again", Where: inX, Modes: writing},
		{Seq: "Ctrl+k", Desc: "Cancel X for this post (LinkedIn is unaffected)", Where: inX, Modes: writing},
		{Seq: "Esc", Desc: "Leave it for later (it waits in Queue)", Where: inX, Modes: writing},

		{Seq: "Ctrl+Enter", Desc: "Post it now, or schedule it when it has a time", Where: inWeb, Modes: writing},
		{Seq: "Ctrl+s", Desc: "Save it as a draft", Where: inWeb, Modes: writing},
		{Seq: "Esc", Desc: "Leave the open window", Where: inWeb, Modes: writing},
	}).
	Group("Space p", "Post").
	Group("Space n", "Networks").
	Where(inCompose, "Compose").
	Where(inLists, "Queue, Drafts, Posted").
	Where(inApprove, "Approving an agent's post").
	Where(inSettings, "Settings").
	Where(inX, "While X waits for your click").
	Where(inWeb, "vaikhari web")
