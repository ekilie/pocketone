package pitr

import (
	"context"
	"strings"

	restorev1 "github.com/pockethost/pocketone/ext/contracts/restore/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// restorer is restore/v1 over one app's history: any moment inside the window.
type restorer struct{ a *appPITR }

func (r restorer) Points(ctx context.Context, _ extkit.Auth) (restorev1.Points, error) {
	st, err := r.a.Status(ctx)
	if err != nil {
		return restorev1.Points{}, err
	}
	p := restorev1.Points{Provider: Name, Kind: restorev1.Continuous, Restoring: st.Restoring || st.Pending != nil,
		Details: map[string]any{"destination": st.Destination, "label": st.Label, "size": st.Size, "retention": st.Retention}}
	if st.Window != nil {
		p.Window = &restorev1.Window{Earliest: st.Window.Earliest, Latest: st.Window.Latest}
	}
	if lr := st.LastRestore; lr != nil {
		if lr.Error == "" {
			p.Last = &restorev1.Restored{Asked: lr.At, Point: lr.At, Done: lr.RestoredAt}
		} else {
			p.Details["lastError"] = lr.Error
		}
	}
	if st.Pending != nil {
		p.Details["pending"] = st.Pending
	}
	return p, nil
}

func (r restorer) RestoreTo(ctx context.Context, _ extkit.Auth, in restorev1.RestoreInput) (restorev1.RestoreResult, error) {
	if err := r.a.Restore(ctx, in.At); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "already running"):
			return restorev1.RestoreResult{}, restorev1.ErrBusy
		case strings.Contains(msg, "no history") || strings.HasPrefix(msg, "history starts"):
			return restorev1.RestoreResult{}, &extkit.Error{Code: restorev1.ErrNoPoint.Code, Message: msg, Status: restorev1.ErrNoPoint.Status}
		}
		return restorev1.RestoreResult{}, &extkit.Error{Code: "bad_request", Message: msg, Status: 400}
	}
	return restorev1.RestoreResult{Restoring: true, Point: in.At, Message: "restoring; the app restarts to apply it"}, nil
}
