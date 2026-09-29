// omniStatus
// Copyright (C) 2024 omniStatus Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

//go:build windows

// This file implements the optional floating "details" window - a small
// borderless, always-on-top panel showing the full per-platform status
// breakdown persistently, since the tray icon itself can only show one
// dot (see main.go's package doc comment for why). User-requested
// addition after the first real-Windows test: the dot alone wasn't
// informative enough at a glance.
//
// THREADING MODEL (read this before changing anything here):
//
// Windows message loops are thread-affine: a window's messages are only
// ever pumped by the OS thread that created it, and GDI painting is only
// safe on that same thread. systray's own Windows backend (see
// fyne.io/systray's systray_windows.go, nativeStart/doNativeTick) already
// runs its tray icon's message loop on its own dedicated goroutine,
// separate from main()'s - confirmed by reading its source before writing
// this. This file follows the exact same pattern for the details window:
// startDetailsWindow launches a dedicated goroutine, locks it to its OS
// thread for its entire lifetime (runtime.LockOSThread), creates the
// window on that thread, and runs an independent GetMessage/
// TranslateMessage/DispatchMessage loop there for as long as the app
// runs. This is standard, safe Win32 practice - each thread owns and
// pumps only the messages for the windows it created, with no possible
// conflict between threads - not an improvised workaround.
//
// Because of this, nothing outside this file's own goroutine may touch
// the window directly (no GDI calls, no ShowWindow, nothing) from other
// goroutines (menu click handlers, the poll loop, etc.). Cross-goroutine
// communication goes through PostMessage with custom WM_APP+N messages,
// handled inside wndProc on the window's own thread - see
// ToggleDetailsWindow and SetDetailsLines below.
package main

import (
	"image/color"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy   = 0x0002
	wmPaint     = 0x000F
	wmNCHitTest = 0x0084
	wmApp       = 0x8000
	wmToggle    = wmApp + 1 // custom: toggle show/hide
	wmSetLines  = wmApp + 2 // custom: new data is available, repaint

	htCaption = 2 // returned from WM_NCHITTEST to make the whole window draggable, see wndProc

	idcArrow = 32512 // standard arrow cursor, as a MAKEINTRESOURCE ordinal

	wsPopup = 0x80000000

	wsExTopMost    = 0x00000008
	wsExToolWindow = 0x00000080 // hides it from the taskbar/alt-tab

	swHide = 0
	swShow = 5

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	bkModeTransparent = 1

	dtSingleLine = 0x00000020
	dtVCenter    = 0x00000004
	dtLeft       = 0x00000000

	detailsWindowWidth   = 260 // vertical layout: fixed width, height grows with line count
	detailsLineHeight    = 26
	detailsCellWidth     = 130 // horizontal layout fallback only - see measureCellWidth for the real (content-measured) width
	detailsRowHeight     = 36  // horizontal layout: fixed window height (one row)
	detailsIconTextGap   = 6   // horizontal layout: gap between a cell's icon and its text
	detailsCellGap       = 16  // horizontal layout: gap between adjacent cells
	detailsPadding       = 10
	detailsScreenMarginX = 12
	detailsScreenMarginY = 60 // rough clearance above the taskbar - not taskbar-aware, see note below
)

var (
	u32 = windows.NewLazySystemDLL("user32.dll")
	g32 = windows.NewLazySystemDLL("gdi32.dll")

	pRegisterClassEx  = u32.NewProc("RegisterClassExW")
	pCreateWindowEx   = u32.NewProc("CreateWindowExW")
	pDefWindowProc    = u32.NewProc("DefWindowProcW")
	pGetMessage       = u32.NewProc("GetMessageW")
	pTranslateMessage = u32.NewProc("TranslateMessage")
	pDispatchMessage  = u32.NewProc("DispatchMessageW")
	pShowWindow       = u32.NewProc("ShowWindow")
	pIsWindowVisible  = u32.NewProc("IsWindowVisible")
	pPostMessage      = u32.NewProc("PostMessageW")
	pInvalidateRect   = u32.NewProc("InvalidateRect")
	pBeginPaint       = u32.NewProc("BeginPaint")
	pEndPaint         = u32.NewProc("EndPaint")
	pFrameRect        = u32.NewProc("FrameRect")
	pDrawText         = u32.NewProc("DrawTextW")
	pGetSystemMetrics = u32.NewProc("GetSystemMetrics")
	pSetWindowPos     = u32.NewProc("SetWindowPos")
	pLoadCursor       = u32.NewProc("LoadCursorW")
	pGetDC            = u32.NewProc("GetDC")
	pReleaseDC        = u32.NewProc("ReleaseDC")
	pGetModuleHandle  = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")

	pCreateSolidBrush      = g32.NewProc("CreateSolidBrush")
	pCreateFontW           = g32.NewProc("CreateFontW")
	pSelectObject          = g32.NewProc("SelectObject")
	pDeleteObject          = g32.NewProc("DeleteObject")
	pGetTextExtentPoint32W = g32.NewProc("GetTextExtentPoint32W")
	pSetTextColor          = g32.NewProc("SetTextColor")
	pSetBkMode             = g32.NewProc("SetBkMode")
)

// wndClassEx mirrors the Win32 WNDCLASSEXW struct layout exactly (field
// order and sizes matter - this is passed by raw pointer to
// RegisterClassExW).
type wndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background windows.Handle
	MenuName, ClassName                *uint16
	IconSm                             windows.Handle
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type paintStruct struct {
	HDC         uintptr
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type msg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// sizeXY mirrors the Win32 SIZE struct, used to receive the result of
// GetTextExtentPoint32W.
type sizeXY struct {
	CX, CY int32
}

// detailsLine is one row of the floating window: a colored icon character
// (drawn in its own font, if configured) followed by plain status text.
type detailsLine struct {
	Icon      string
	IconColor color.RGBA
	IconFont  string // font family name, "" = system default
	Text      string
}

var (
	// detailsHandle is written once by startDetailsWindow's goroutine
	// after CreateWindowExW succeeds, and read by any goroutine wanting
	// to post a message to it. atomic since it crosses goroutines with no
	// other synchronization.
	detailsHandle atomic.Uint64

	detailsMu    sync.Mutex
	detailsLines []detailsLine

	// detailsLayout is "vertical" (default) or "horizontal" - set once at
	// startup from config (display.tray_layout) via SetDetailsLayout,
	// before the window is shown. Not hot-reloadable, same as the rest of
	// this app's config (see main.go's package doc comment).
	detailsLayout atomic.Value

	fontCache   = map[string]windows.Handle{}
	fontCacheMu sync.Mutex
)

// startDetailsWindow creates the (initially hidden) floating status
// window and runs its message loop for the lifetime of the app. Must be
// called exactly once, in its own goroutine (it never returns) - e.g.
// `go startDetailsWindow()` from onReady.
func startDetailsWindow() {
	runtime.LockOSThread()

	instance, _, _ := pGetModuleHandle.Call(0)

	className, _ := windows.UTF16PtrFromString("omnistatusDetailsWindow")
	windowName, _ := windows.UTF16PtrFromString("omniStatus")

	bgBrush, _, _ := pCreateSolidBrush.Call(uintptr(colorRef(color.RGBA{R: 24, G: 24, B: 24, A: 255})))

	// A window class with no cursor set leaves Windows showing whatever
	// cursor was last active over it - observed live as the spinning
	// "busy" wait cursor, giving a false impression of instability. Fix:
	// explicitly load and set the standard arrow, same as systray's own
	// window class does for its own (invisible) message window.
	cursor, _, _ := pLoadCursor.Call(0, uintptr(idcArrow))

	wc := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		Style:      csHRedraw | csVRedraw,
		WndProc:    windows.NewCallback(detailsWndProc),
		Instance:   windows.Handle(instance),
		Cursor:     windows.Handle(cursor),
		Background: windows.Handle(bgBrush),
		ClassName:  className,
	}
	if ret, _, _ := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return // nothing sensible to do if even this fails; details window just won't work
	}

	x, y, w, h := detailsWindowRect(1) // sized for 1 line initially; resized on first data update... see note in resizeDetailsWindow

	hwnd, _, _ := pCreateWindowEx.Call(
		uintptr(wsExTopMost|wsExToolWindow),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(wsPopup),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0,
		instance,
		0,
	)
	if hwnd == 0 {
		return
	}
	detailsHandle.Store(uint64(hwnd))

	var m msg
	for {
		ret, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// detailsWindowRect computes the window's default initial position
// (bottom-right of the primary display, roughly clear of the taskbar) and
// its size for the given number of lines. The x/y it returns are only
// used once, at window creation - resizeDetailsWindow deliberately
// ignores them afterward (SWP_NOMOVE) so a user drag sticks instead of
// snapping back on the next data refresh. Not taskbar-aware (doesn't
// query the actual taskbar rect via the Shell API) - detailsScreenMarginY
// is a fixed approximation, good enough for the default bottom taskbar
// case but may sit under/over a taskbar docked elsewhere or resized.
func detailsWindowRect(lineCount int) (x, y, w, h int32) {
	screenW, _, _ := pGetSystemMetrics.Call(0) // SM_CXSCREEN
	screenH, _, _ := pGetSystemMetrics.Call(1) // SM_CYSCREEN

	if currentLayout() == "horizontal" {
		w = int32(detailsPadding*2 + lineCount*detailsCellWidth)
		h = detailsRowHeight
	} else {
		w = detailsWindowWidth
		h = int32(detailsPadding*2 + lineCount*detailsLineHeight)
	}
	x = int32(screenW) - w - detailsScreenMarginX
	y = int32(screenH) - h - detailsScreenMarginY
	return x, y, w, h
}

// SetDetailsLayout sets the window's layout ("vertical" or "horizontal")
// for all future paints/resizes. Call once at startup, before the window
// is shown - not meant to change at runtime.
func SetDetailsLayout(layout string) {
	detailsLayout.Store(layout)
}

// currentLayout returns the configured layout, defaulting to "vertical"
// if SetDetailsLayout was never called or given an empty string.
func currentLayout() string {
	if v, ok := detailsLayout.Load().(string); ok && v != "" {
		return v
	}
	return "vertical"
}

// ToggleDetailsWindow shows the window if hidden, hides it if shown.
// Safe to call from any goroutine.
func ToggleDetailsWindow() {
	if hwnd := detailsHandle.Load(); hwnd != 0 {
		pPostMessage.Call(uintptr(hwnd), wmToggle, 0, 0)
	}
}

// SetDetailsLines updates what the window shows and, if it's currently
// visible, triggers a repaint. Safe to call from any goroutine - refresh()
// in main.go calls this every time it fetches fresh presence data.
func SetDetailsLines(lines []detailsLine) {
	detailsMu.Lock()
	detailsLines = lines
	detailsMu.Unlock()

	if hwnd := detailsHandle.Load(); hwnd != 0 {
		pPostMessage.Call(uintptr(hwnd), wmSetLines, 0, 0)
	}
}

func detailsWndProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmToggle:
		visible, _, _ := pIsWindowVisible.Call(uintptr(hwnd))
		if visible != 0 {
			pShowWindow.Call(uintptr(hwnd), swHide)
		} else {
			resizeDetailsWindow(hwnd)
			pShowWindow.Call(uintptr(hwnd), swShow)
			pInvalidateRect.Call(uintptr(hwnd), 0, 1)
		}
		return 0

	case wmSetLines:
		resizeDetailsWindow(hwnd)
		pInvalidateRect.Call(uintptr(hwnd), 0, 1)
		return 0

	case wmNCHitTest:
		// Makes the entire window draggable like a title bar, without
		// manually tracking mouse deltas: returning HTCAPTION here tells
		// Windows "treat any click-drag on this window as if the user
		// grabbed the title bar," which gets native OS window-dragging
		// for free. There's no title bar/close button, so this replaces
		// the click-to-dismiss behavior an earlier version of this file
		// had - dismiss is menu-item-only now, see wmToggle.
		return htCaption

	case wmPaint:
		paintDetailsWindow(hwnd)
		return 0

	case wmDestroy:
		return 0
	}

	ret, _, _ := pDefWindowProc.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return ret
}

// resizeDetailsWindow adjusts the window's height (and re-anchors its
// bottom-right position) to fit the current line count, reusing the same
// detailsWindowRect helper used at creation so the positioning logic
// lives in exactly one place.
func resizeDetailsWindow(hwnd windows.Handle) {
	detailsMu.Lock()
	lines := append([]detailsLine(nil), detailsLines...)
	detailsMu.Unlock()
	n := len(lines)
	if n == 0 {
		n = 1
	}

	var w, h int32
	if currentLayout() == "horizontal" && len(lines) > 0 {
		// Horizontal cells are sized to their actual content (see
		// measureCellWidth), not a fixed guess - needs a DC to measure
		// text with, same as painting does, just outside a paint cycle.
		hdc, _, _ := pGetDC.Call(uintptr(hwnd))
		w = detailsPadding * 2
		for i, line := range lines {
			if i > 0 {
				w += detailsCellGap
			}
			w += measureCellWidth(hdc, line)
		}
		h = detailsRowHeight
		pReleaseDC.Call(uintptr(hwnd), hdc)
	} else {
		_, _, rw, rh := detailsWindowRect(n)
		w, h = rw, rh
	}

	// SWP_NOMOVE: resize only, leave the window wherever it currently is.
	// Without this, a user drag (now possible via WM_NCHITTEST above)
	// would get silently undone back to the bottom-right corner the next
	// time refresh() runs (every 15s, or after any click) and calls this.
	const swpNoZOrder = 0x0004
	const swpNoMove = 0x0002
	pSetWindowPos.Call(uintptr(hwnd), 0, 0, 0, uintptr(w), uintptr(h), swpNoZOrder|swpNoMove)
}

func paintDetailsWindow(hwnd windows.Handle) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}

	pSetBkMode.Call(hdc, bkModeTransparent)

	// Thin light-gray border around the whole client area, for definition
	// against whatever's on the desktop behind it.
	borderBrush, _, _ := pCreateSolidBrush.Call(uintptr(colorRef(color.RGBA{R: 90, G: 90, B: 90, A: 255})))
	defer pDeleteObject.Call(borderBrush)
	full := rect{Left: 0, Top: 0, Right: ps.RcPaint.Right, Bottom: ps.RcPaint.Bottom}
	pFrameRect.Call(hdc, uintptr(unsafe.Pointer(&full)), borderBrush)

	detailsMu.Lock()
	lines := append([]detailsLine(nil), detailsLines...)
	detailsMu.Unlock()

	textFont := getOrCreateFont("", 15)
	horizontal := currentLayout() == "horizontal"
	cellLeft := int32(detailsPadding) // horizontal layout only - advances per cell below

	for i, line := range lines {
		iconFont := getOrCreateFont(line.IconFont, 15)
		iconWidth := measureText(hdc, iconFont, line.Icon)

		var iconRect, textRect rect
		if horizontal {
			// Content-fit: the icon column is exactly as wide as the
			// measured glyph, text starts right after it (plus a small
			// gap), and the next cell starts after this one's total
			// measured width - kept in sync with resizeDetailsWindow's
			// measureCellWidth, which sized the window to match.
			cellWidth := measureCellWidth(hdc, line)
			iconRect = rect{Left: cellLeft, Top: 0, Right: cellLeft + iconWidth, Bottom: detailsRowHeight}
			textRect = rect{Left: cellLeft + iconWidth + detailsIconTextGap, Top: 0, Right: cellLeft + cellWidth, Bottom: detailsRowHeight}
			cellLeft += cellWidth + detailsCellGap
		} else {
			top := int32(detailsPadding + i*detailsLineHeight)
			lineRect := rect{
				Left:   detailsPadding,
				Top:    top,
				Right:  detailsWindowWidth - detailsPadding,
				Bottom: top + detailsLineHeight,
			}
			iconRect = lineRect
			iconRect.Right = iconRect.Left + 24
			textRect = lineRect
			textRect.Left += 28
		}

		// Icon glyph, in its own font/color if configured.
		prev, _, _ := pSelectObject.Call(hdc, uintptr(iconFont))
		pSetTextColor.Call(hdc, uintptr(colorRef(line.IconColor)))
		iconPtr, _ := windows.UTF16PtrFromString(line.Icon)
		pDrawText.Call(hdc, uintptr(unsafe.Pointer(iconPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&iconRect)), dtSingleLine|dtVCenter|dtLeft)
		pSelectObject.Call(hdc, prev)

		// Status text, plain light gray, to the right of the icon.
		pSelectObject.Call(hdc, uintptr(textFont))
		pSetTextColor.Call(hdc, uintptr(colorRef(color.RGBA{R: 220, G: 220, B: 220, A: 255})))
		textPtr, _ := windows.UTF16PtrFromString(line.Text)
		pDrawText.Call(hdc, uintptr(unsafe.Pointer(textPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&textRect)), dtSingleLine|dtVCenter|dtLeft)
	}
}

// measureText returns the rendered width, in pixels, of text drawn with
// font on the given device context - used to size horizontal-layout cells
// to their actual content instead of a fixed guess. Temporarily selects
// font into hdc and restores whatever was selected before, so it's safe
// to call in the middle of painting without disturbing anything else.
//
// Deliberately measures the UTF-16-encoded length, not len([]rune(text)):
// Nerd Font private-use-area glyphs (like the ones this project's own
// config uses) commonly sit above U+FFFF, which UTF-16 represents as a
// 2-unit surrogate pair - using the rune count would undercount those and
// throw the measurement off by one code unit per such glyph.
func measureText(hdc uintptr, font windows.Handle, text string) int32 {
	if text == "" {
		return 0
	}

	prev, _, _ := pSelectObject.Call(hdc, uintptr(font))
	defer pSelectObject.Call(hdc, prev)

	utf16Text, err := windows.UTF16FromString(text)
	if err != nil || len(utf16Text) < 2 {
		return 0
	}
	count := len(utf16Text) - 1 // UTF16FromString includes a trailing NUL; exclude it

	var sz sizeXY
	pGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&utf16Text[0])), uintptr(count), uintptr(unsafe.Pointer(&sz)))
	return sz.CX
}

// measureCellWidth returns how wide one horizontal-layout cell needs to be
// to fit line's icon and text (each in their own font) with the gap
// between them, but no text at all if line.Text is empty (e.g.
// display.tray_show_state: false with a custom icon set - see main.go's
// refresh()).
func measureCellWidth(hdc uintptr, line detailsLine) int32 {
	iconFont := getOrCreateFont(line.IconFont, 15)
	textFont := getOrCreateFont("", 15)

	w := measureText(hdc, iconFont, line.Icon)
	if textW := measureText(hdc, textFont, line.Text); textW > 0 {
		w += detailsIconTextGap + textW
	}
	return w
}

// getOrCreateFont returns a cached font handle for the given family
// name/size, creating and caching it on first use. name == "" uses the
// system default UI-ish font (empty face name lets GDI pick).
func getOrCreateFont(name string, size int32) windows.Handle {
	key := name
	fontCacheMu.Lock()
	defer fontCacheMu.Unlock()
	if h, ok := fontCache[key]; ok {
		return h
	}

	namePtr, _ := windows.UTF16PtrFromString(name)
	h, _, _ := pCreateFontW.Call(
		uintptr(size), 0, 0, 0,
		400, // FW_NORMAL
		0, 0, 0,
		1, // DEFAULT_CHARSET - required for non-Latin/symbol glyphs (Nerd Fonts) to be selectable at all
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	handle := windows.Handle(h)
	fontCache[key] = handle
	return handle
}
