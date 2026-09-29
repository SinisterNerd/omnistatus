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
	"image/color"
	"strconv"
	"strings"
)

// namedColors is a modest set of common color names - enough to cover
// what this project's own config.example.yaml and README examples
// actually use. Intentionally not exhaustive; an unrecognized name just
// falls through to the caller's fallback color rather than erroring.
var namedColors = map[string]color.RGBA{
	"black":   {R: 0, G: 0, B: 0, A: 255},
	"red":     {R: 255, G: 0, B: 0, A: 255},
	"green":   {R: 0, G: 255, B: 0, A: 255},
	"yellow":  {R: 255, G: 255, B: 0, A: 255},
	"blue":    {R: 0, G: 0, B: 255, A: 255},
	"magenta": {R: 255, G: 0, B: 255, A: 255},
	"cyan":    {R: 0, G: 255, B: 255, A: 255},
	"white":   {R: 255, G: 255, B: 255, A: 255},
	"gray":    {R: 128, G: 128, B: 128, A: 255},
	"grey":    {R: 128, G: 128, B: 128, A: 255},
	"orange":  {R: 255, G: 165, B: 0, A: 255},
}

// parseColor converts a config color value (as config.PlatformConfig's
// ColorFor returns it) into an RGBA for native GDI rendering. Understands
// hex ("#rrggbb") and the common names in namedColors. Deliberately does
// NOT resolve tmux's 256-color palette indices ("colourNNN") - that would
// need embedding the full xterm 256-color table for a case this project's
// own config doesn't currently use (it uses hex) - falls back to
// `fallback` instead, same as any other unrecognized value.
func parseColor(s string, fallback color.RGBA) color.RGBA {
	s = strings.TrimSpace(strings.ToLower(s))

	if strings.HasPrefix(s, "#") && len(s) == 7 {
		r, errR := strconv.ParseUint(s[1:3], 16, 8)
		g, errG := strconv.ParseUint(s[3:5], 16, 8)
		b, errB := strconv.ParseUint(s[5:7], 16, 8)
		if errR == nil && errG == nil && errB == nil {
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		}
	}

	if c, ok := namedColors[s]; ok {
		return c
	}

	return fallback
}

// colorRef packs an RGBA into a Win32 COLORREF (0x00BBGGRR - note the
// byte order is reversed from the usual RGB reading order, and alpha is
// not part of a COLORREF at all).
func colorRef(c color.RGBA) uint32 {
	return uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16
}
