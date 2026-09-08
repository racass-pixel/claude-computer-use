//go:build windows

package uia

import (
	"fmt"
	"regexp"
	"runtime"
	"time"

	"github.com/go-ole/go-ole"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type UIA struct {
	req  chan func()
	auto comObj
}

// New starts the COM thread. Every UIA call runs there via run().
func New() (*UIA, error) {
	u := &UIA{req: make(chan func())}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
			ready <- err
			return
		}
		a, err := createAutomation()
		if err != nil {
			ready <- err
			return
		}
		u.auto = a
		ready <- nil
		for f := range u.req {
			f()
		}
		a.release()
		ole.CoUninitialize()
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return u, nil
}

func (u *UIA) run(f func() error) error {
	done := make(chan error, 1)
	select {
	case u.req <- func() { done <- f() }:
	case <-time.After(8 * time.Second):
		return fmt.Errorf("ui automation is busy (a previous query has not returned)")
	}
	select {
	case err := <-done:
		return err
	case <-time.After(8 * time.Second):
		return fmt.Errorf("ui automation timed out; narrow the query (role, window) or use vision")
	}
}

func (u *UIA) Close() { close(u.req) }

func (u *UIA) Find(q platform.FindQuery) ([]platform.Element, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 25
	}
	var re *regexp.Regexp
	if q.Name != "" {
		var err error
		if re, err = regexp.Compile("(?i)" + q.Name); err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
	}
	var out []platform.Element
	err := u.run(func() error {
		var root comObj
		var err error
		if q.Window != 0 {
			root, err = u.auto.elementFromHandle(q.Window)
		} else {
			root, err = u.auto.rootElement()
		}
		if err != nil {
			return err
		}
		defer root.release()

		vTrue := ole.NewVariant(ole.VT_BOOL, -1)
		cond, err := u.auto.propertyCondition(propIsControlElement, &vTrue)
		if err != nil {
			return err
		}
		defer cond.release()
		if q.Role != "" {
			id, ok := RoleID(q.Role)
			if !ok {
				return fmt.Errorf("unknown role %q (Button, Edit, CheckBox, ComboBox, MenuItem, ListItem, TreeItem, TabItem, Hyperlink, Text, Document, Window, Pane...)", q.Role)
			}
			vRole := ole.NewVariant(ole.VT_I4, int64(id))
			roleCond, err := u.auto.propertyCondition(propControlType, &vRole)
			if err != nil {
				return err
			}
			defer roleCond.release()
			both, err := u.auto.andCondition(cond, roleCond)
			if err != nil {
				return err
			}
			defer both.release()
			cond = both
		}
		cache, err := u.auto.cacheRequest(propName, propControlType, propBoundingRectangle, propAutomationID, propIsEnabled, propHasKeyboardFocus, propValueValue, propIsOffscreen)
		if err != nil {
			return err
		}
		defer cache.release()
		arr, err := root.findAllBuildCache(treeScopeDescendants, cond, cache)
		if err != nil {
			return err
		}
		defer arr.release()
		n := arr.length()
		for i := int32(0); i < n && len(out) < limit; i++ {
			el, err := arr.element(i)
			if err != nil {
				continue
			}
			name := el.cachedString(propName)
			autoID := el.cachedString(propAutomationID)
			if (re != nil && !re.MatchString(name)) || (q.AutomationID != "" && autoID != q.AutomationID) {
				el.release()
				continue
			}
			l, t, w, h, ok := el.rect(vtElemGetCachedPropertyValue)
			if !ok || w <= 0 || h <= 0 {
				el.release()
				continue
			}
			out = append(out, platform.Element{
				Name: name, Role: RoleName(el.cachedInt(propControlType)), AutomationID: autoID,
				Value:   el.cachedString(propValueValue),
				Rect:    geom.Rect{X: int(l), Y: int(t), W: int(w), H: int(h)},
				Enabled: el.cachedBool(propIsEnabled), Focused: el.cachedBool(propHasKeyboardFocus),
				Offscreen: el.cachedBool(propIsOffscreen), Ref: el,
			})
		}
		return nil
	})
	return out, err
}

func (u *UIA) Rect(ref any) (geom.Rect, error) {
	el, ok := ref.(comObj)
	if !ok {
		return geom.Rect{}, fmt.Errorf("bad element ref")
	}
	var r geom.Rect
	err := u.run(func() error {
		l, t, w, h, ok := el.rect(vtElemGetCurrentPropertyValue)
		if !ok {
			return fmt.Errorf("element has no bounding rectangle any more")
		}
		r = geom.Rect{X: int(l), Y: int(t), W: int(w), H: int(h)}
		return nil
	})
	return r, err
}

func (u *UIA) Release(refs []any) {
	_ = u.run(func() error {
		for _, r := range refs {
			if el, ok := r.(comObj); ok {
				el.release()
			}
		}
		return nil
	})
}

var _ platform.Accessibility = (*UIA)(nil)
