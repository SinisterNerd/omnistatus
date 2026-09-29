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

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// fatalError shows a native Windows message box with the given text, then
// exits the process. This exists because this binary is built with
// -ldflags="-H=windowsgui" (see Makefile - needed so launching it doesn't
// pop up a console window alongside the tray icon), which means there is
// no console for a plain log.Fatalf to write to: without this, a startup
// failure (e.g. a missing config file) just makes the process silently
// disappear from Task Manager a moment after double-clicking it, with
// zero indication of why - confirmed live, this is exactly what happened
// the first time this app was actually run on real Windows. Every fatal
// startup error must go through this, not log.Fatalf, or it vanishes
// with no feedback the same way.
func fatalError(title, message string) {
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err == nil {
		if messagePtr, err := windows.UTF16PtrFromString(message); err == nil {
			_, _ = windows.MessageBox(0, messagePtr, titlePtr, windows.MB_OK|windows.MB_ICONERROR)
		}
	}
	os.Exit(1)
}
