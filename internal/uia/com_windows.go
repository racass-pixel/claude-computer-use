//go:build windows

package uia

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var (
	clsidCUIAutomation = ole.NewGUID("{ff48dba4-60ef-4201-aa87-54103eef594e}")
	iidIUIAutomation   = ole.NewGUID("{30cbe57d-d9d0-452a-ab13-7ac5ac4825ee}")
)

const (
	propBoundingRectangle = 30001
	propControlType       = 30003
	propName              = 30005
	propHasKeyboardFocus  = 30008
	propIsEnabled         = 30010
	propAutomationID      = 30011
	propIsControlElement  = 30016
	propIsOffscreen       = 30022
	propValueValue        = 30045

	treeScopeDescendants = 4
)

// Vtable slots from UIAutomationClient.h (IUnknown occupies 0-2). If any call returns E_NOTIMPL/garbage,
// compare against the interface definitions in github.com/sjuhan/w32uiautomation and fix the slot.
const (
	vtAutoGetRootElement      = 5
	vtAutoElementFromHandle   = 6
	vtAutoCreateCacheRequest  = 20
	vtAutoCreateTrueCondition = 21
	vtAutoCreatePropertyCond  = 23
	vtAutoCreateAndCondition  = 25

	vtElemFindAllBuildCache       = 8
	vtElemGetCurrentPropertyValue = 10
	vtElemGetCachedPropertyValue  = 12

	vtArrayGetLength  = 3
	vtArrayGetElement = 4

	vtCacheAddProperty = 3
)

type comObj struct{ *ole.IUnknown }

func (c comObj) call(slot int, args ...uintptr) uintptr {
	vt := (*[64]uintptr)(unsafe.Pointer(c.RawVTable))[slot]
	full := append([]uintptr{uintptr(unsafe.Pointer(c.IUnknown))}, args...)
	hr, _, _ := syscall.SyscallN(vt, full...)
	return hr
}

func (c comObj) release() {
	if c.IUnknown != nil {
		c.IUnknown.Release()
	}
}

func hrErr(what string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", what, uint32(hr))
	}
	return nil
}

func outObj(c comObj, what string, slot int, args ...uintptr) (comObj, error) {
	var p *ole.IUnknown
	hr := c.call(slot, append(args, uintptr(unsafe.Pointer(&p)))...)
	if err := hrErr(what, hr); err != nil {
		return comObj{}, err
	}
	if p == nil {
		return comObj{}, fmt.Errorf("%s: null result", what)
	}
	return comObj{p}, nil
}

func createAutomation() (comObj, error) {
	unk, err := ole.CreateInstance(clsidCUIAutomation, iidIUIAutomation)
	if err != nil {
		return comObj{}, fmt.Errorf("CUIAutomation: %w", err)
	}
	return comObj{unk}, nil
}

func (a comObj) rootElement() (comObj, error) {
	return outObj(a, "GetRootElement", vtAutoGetRootElement)
}

func (a comObj) elementFromHandle(hwnd uintptr) (comObj, error) {
	return outObj(a, "ElementFromHandle", vtAutoElementFromHandle, hwnd)
}

func (a comObj) trueCondition() (comObj, error) {
	return outObj(a, "CreateTrueCondition", vtAutoCreateTrueCondition)
}

// propertyCondition passes the VARIANT by value; on x64 that is a pointer to a caller-owned copy.
func (a comObj) propertyCondition(prop int32, v *ole.VARIANT) (comObj, error) {
	return outObj(a, "CreatePropertyCondition", vtAutoCreatePropertyCond, uintptr(prop), uintptr(unsafe.Pointer(v)))
}

func (a comObj) andCondition(x, y comObj) (comObj, error) {
	return outObj(a, "CreateAndCondition", vtAutoCreateAndCondition, uintptr(unsafe.Pointer(x.IUnknown)), uintptr(unsafe.Pointer(y.IUnknown)))
}

func (a comObj) cacheRequest(props ...int32) (comObj, error) {
	cr, err := outObj(a, "CreateCacheRequest", vtAutoCreateCacheRequest)
	if err != nil {
		return comObj{}, err
	}
	for _, p := range props {
		if err := hrErr("CacheRequest.AddProperty", cr.call(vtCacheAddProperty, uintptr(p))); err != nil {
			cr.release()
			return comObj{}, err
		}
	}
	return cr, nil
}

func (e comObj) findAllBuildCache(scope int32, cond, cache comObj) (comObj, error) {
	return outObj(e, "FindAllBuildCache", vtElemFindAllBuildCache, uintptr(scope), uintptr(unsafe.Pointer(cond.IUnknown)), uintptr(unsafe.Pointer(cache.IUnknown)))
}

func (arr comObj) length() int32 {
	var n int32
	arr.call(vtArrayGetLength, uintptr(unsafe.Pointer(&n)))
	return n
}

func (arr comObj) element(i int32) (comObj, error) {
	return outObj(arr, "ElementArray.GetElement", vtArrayGetElement, uintptr(i))
}

func (e comObj) prop(slot int, prop int32) (ole.VARIANT, error) {
	var v ole.VARIANT
	ole.VariantInit(&v)
	hr := e.call(slot, uintptr(prop), uintptr(unsafe.Pointer(&v)))
	return v, hrErr("GetPropertyValue", hr)
}

func (e comObj) cachedString(prop int32) string {
	v, err := e.prop(vtElemGetCachedPropertyValue, prop)
	if err != nil {
		return ""
	}
	defer v.Clear()
	if v.VT == ole.VT_BSTR {
		return v.ToString()
	}
	return ""
}

func (e comObj) cachedBool(prop int32) bool {
	v, err := e.prop(vtElemGetCachedPropertyValue, prop)
	if err != nil {
		return false
	}
	defer v.Clear()
	b, _ := v.Value().(bool)
	return b
}

func (e comObj) cachedInt(prop int32) int32 {
	v, err := e.prop(vtElemGetCachedPropertyValue, prop)
	if err != nil {
		return 0
	}
	defer v.Clear()
	switch x := v.Value().(type) {
	case int32:
		return x
	case int64:
		return int32(x)
	}
	return 0
}

// rectFromVariant reads a UIA rectangle (SAFEARRAY of 4 doubles: left, top, width, height).
// The caller owns the VARIANT and must Clear it; we must NOT release the SafeArray here
// because v.Clear() releases it — a double-free causes heap corruption.
func rectFromVariant(v *ole.VARIANT) (l, t, w, h float64, ok bool) {
	arr := v.ToArray()
	if arr == nil {
		return
	}
	vals := arr.ToValueArray()
	if len(vals) != 4 {
		return
	}
	f := func(x any) float64 {
		switch n := x.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		}
		return 0
	}
	return f(vals[0]), f(vals[1]), f(vals[2]), f(vals[3]), true
}

func (e comObj) rect(slot int) (l, t, w, h float64, ok bool) {
	v, err := e.prop(slot, propBoundingRectangle)
	if err != nil {
		return
	}
	defer v.Clear()
	return rectFromVariant(&v)
}
