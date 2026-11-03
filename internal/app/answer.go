package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Theyashsawarkar/vaikhari/internal/post"
)

// ApproveAs is Approve with the answer changed first: only some of the
// networks it asked for (none: all of them), or another time (0: as asked,
// below 0: now).
func (a *App) ApproveAs(ctx context.Context, id int64, text *string, networks []string, sendAt int64, interactive bool) (post.Post, error) {
	if len(networks) > 0 || sendAt != 0 {
		p, err := a.Store.Get(id)
		if err != nil {
			return p, err
		}
		if p.Status != post.Approval {
			return p, fmt.Errorf("#%d is %s, not waiting for approval", id, p.Status)
		}
		if len(networks) > 0 {
			var keep []string
			for _, n := range post.Normalize(networks) {
				if p.On(n) {
					keep = append(keep, n)
				}
			}
			if len(keep) == 0 {
				return p, fmt.Errorf("#%d isn't for %v", id, networks)
			}
			p.Networks = keep
		}
		if sendAt != 0 {
			p.SendAt = max(sendAt, 0)
		}
		if text != nil { // checked before anything is written: a refused answer changes nothing
			check := p
			check.Text = *text
			if msg := post.Check(check); msg != "" {
				return p, errors.New(msg)
			}
		}
		ok, err := a.Store.Transition(p, post.Approval)
		if err != nil || !ok {
			return p, firstErr(err, fmt.Errorf("#%d was answered elsewhere", id))
		}
	}
	return a.Approve(ctx, id, text, interactive)
}
