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
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// iconSize is the rendered dot's width/height in pixels. 32 rather than the
// classic 16 - Windows scales tray icons for high-DPI displays, and 32
// stays crisp on those without being large enough to look odd on
// standard-DPI ones.
const iconSize = 32

// renderDotIcon draws a solid-colored filled circle and returns it encoded
// as ICO bytes (what systray.SetIcon expects on Windows). No third-party
// image/icon library needed: image/png is stdlib, and the ICO container
// format is simple enough to write by hand - see pngToICO.
func renderDotIcon(c color.RGBA) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))

	center := float64(iconSize) / 2
	radius := center - 1 // 1px margin so the circle doesn't clip at the edges
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, c)
			}
		}
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}

	return pngToICO(pngBuf.Bytes(), iconSize), nil
}

// pngToICO wraps PNG-encoded image data in a minimal single-image ICO
// container. Windows Vista+ accepts PNG-compressed icon resources directly
// (no need to encode as an uncompressed BMP DIB, which would need a lot
// more code) - see MS-ICO / the ICONDIR and ICONDIRENTRY structures.
func pngToICO(pngData []byte, size int) []byte {
	buf := new(bytes.Buffer)

	// ICONDIR (6 bytes)
	binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved, must be 0
	binary.Write(buf, binary.LittleEndian, uint16(1)) // type: 1 = icon
	binary.Write(buf, binary.LittleEndian, uint16(1)) // image count

	// ICONDIRENTRY (16 bytes) - a single entry describing the PNG that follows
	buf.WriteByte(byte(size))                                    // width in pixels (0 means 256, not needed here)
	buf.WriteByte(byte(size))                                    // height in pixels
	buf.WriteByte(0)                                             // color palette count (0 = no palette, true color)
	buf.WriteByte(0)                                             // reserved, must be 0
	binary.Write(buf, binary.LittleEndian, uint16(1))            // color planes
	binary.Write(buf, binary.LittleEndian, uint16(32))           // bits per pixel
	binary.Write(buf, binary.LittleEndian, uint32(len(pngData))) // size of image data
	binary.Write(buf, binary.LittleEndian, uint32(6+16))         // offset to image data (right after the header+entry)

	buf.Write(pngData)
	return buf.Bytes()
}
