package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/png"
	"io"
	"os/exec"
	"time"
)

func writeCloudWebP(path string, img image.Image) error {
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, videoTool("ffmpeg"), "-nostdin", "-y", "-v", "error", "-f", "image2pipe", "-c:v", "png", "-i", "pipe:0", "-frames:v", "1", "-c:v", "libwebp", "-quality", "85", "-compression_level", "4", path)
	cmd.Stdin = &b
	return cmd.Run()
}

// Read only bounded JPEG APP1 metadata; never follow external references.
func jpegOrientation(r io.ReadSeeker) int {
	defer r.Seek(0, io.SeekStart)
	var marker [2]byte
	if _, e := io.ReadFull(r, marker[:]); e != nil || marker != [2]byte{255, 216} {
		return 1
	}
	for n := 0; n < 100; n++ {
		if _, e := io.ReadFull(r, marker[:]); e != nil || marker[0] != 255 || marker[1] == 218 || marker[1] == 217 {
			return 1
		}
		var length [2]byte
		if _, e := io.ReadFull(r, length[:]); e != nil {
			return 1
		}
		size := int(binary.BigEndian.Uint16(length[:])) - 2
		if size < 0 {
			return 1
		}
		b := make([]byte, size)
		if _, e := io.ReadFull(r, b); e != nil {
			return 1
		}
		if marker[1] != 225 || !bytes.HasPrefix(b, []byte("Exif\x00\x00")) {
			continue
		}
		b = b[6:]
		if len(b) < 8 {
			return 1
		}
		var order binary.ByteOrder = binary.LittleEndian
		if string(b[:2]) == "MM" {
			order = binary.BigEndian
		} else if string(b[:2]) != "II" {
			return 1
		}
		if order.Uint16(b[2:4]) != 42 {
			return 1
		}
		offset := int64(order.Uint32(b[4:8]))
		if offset < 8 || offset+2 > int64(len(b)) {
			return 1
		}
		entries := int(order.Uint16(b[offset : offset+2]))
		for i := 0; i < entries; i++ {
			p := int(offset) + 2 + i*12
			if p+12 > len(b) {
				return 1
			}
			if order.Uint16(b[p:p+2]) == 0x112 && order.Uint16(b[p+2:p+4]) == 3 && order.Uint32(b[p+4:p+8]) == 1 {
				v := int(order.Uint16(b[p+8 : p+10]))
				if v >= 1 && v <= 8 {
					return v
				}
				return 1
			}
		}
	}
	return 1
}
func orientPhoto(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
