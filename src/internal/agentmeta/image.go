package agentmeta

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Image caps (Plans/R1.1/agent-images.md#rules): the bytes alone do not bound
// the decoded size, so the header's width and height are bounded too.
const (
	ImageMaxBytes  = 256 << 10
	ImageMaxPixels = 512
)

// Image is an agent's validated picture.
type Image struct {
	Data   []byte
	SHA256 string
	Type   string
}

// Base64 is the image ask's answer body.
func (i Image) Base64() string { return base64.StdEncoding.EncodeToString(i.Data) }

// checkImageName refuses a name that could leave the agent's folder.
func checkImageName(name string) error {
	switch {
	case strings.TrimSpace(name) == "" || name != strings.TrimSpace(name):
		return errors.New("agent.json: image must be a file name inside the agent's folder")
	case filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.Contains(name, "\\"):
		return fmt.Errorf("agent.json: image %q must not be an absolute path", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("agent.json: image %q must not contain empty, . or .. parts", name)
		}
	}
	return nil
}

// LoadImage reads and validates the image a package names. It never follows a
// symlink, in the name or any directory on the way to it.
func LoadImage(dir, name string) (Image, error) {
	if err := checkImageName(name); err != nil {
		return Image{}, err
	}
	at := dir
	parts := strings.Split(name, "/")
	for i, part := range parts {
		at = filepath.Join(at, part)
		fi, err := os.Lstat(at)
		if err != nil {
			return Image{}, fmt.Errorf("agent.json: image %q: %w", name, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return Image{}, fmt.Errorf("agent.json: image %q: a symlink is refused", name)
		}
		if i < len(parts)-1 && !fi.IsDir() {
			return Image{}, fmt.Errorf("agent.json: image %q: %s is not a directory", name, part)
		}
		if i == len(parts)-1 && !fi.Mode().IsRegular() {
			return Image{}, fmt.Errorf("agent.json: image %q is not a regular file", name)
		}
	}
	f, err := OpenFile(at)
	if err != nil {
		return Image{}, fmt.Errorf("agent.json: image %q: %w", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, ImageMaxBytes+1))
	if err != nil {
		return Image{}, fmt.Errorf("agent.json: image %q: %w", name, err)
	}
	img, err := CheckImage(data)
	if err != nil {
		return Image{}, fmt.Errorf("agent.json: image %q: %w", name, err)
	}
	return img, nil
}

// CheckImage applies the caps to bytes from any source: the type comes from the
// first bytes, never a file name.
func CheckImage(data []byte) (Image, error) {
	if len(data) > ImageMaxBytes {
		return Image{}, fmt.Errorf("over %d KiB", ImageMaxBytes>>10)
	}
	kind, w, h, err := imageSize(data)
	if err != nil {
		return Image{}, err
	}
	if w <= 0 || h <= 0 {
		return Image{}, errors.New("has no size in its header")
	}
	if w > ImageMaxPixels || h > ImageMaxPixels {
		return Image{}, fmt.Errorf("is %d×%d, over %d×%d", w, h, ImageMaxPixels, ImageMaxPixels)
	}
	if w != h {
		return Image{}, fmt.Errorf("is %d×%d, not square", w, h)
	}
	sum := sha256.Sum256(data)
	return Image{Data: data, SHA256: hex.EncodeToString(sum[:]), Type: kind}, nil
}

func imageSize(b []byte) (string, int, int, error) {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		if len(b) < 24 || string(b[12:16]) != "IHDR" {
			return "", 0, 0, errors.New("is a PNG without IHDR")
		}
		return "image/png", int(binary.BigEndian.Uint32(b[16:20])), int(binary.BigEndian.Uint32(b[20:24])), nil
	case bytes.HasPrefix(b, []byte{0xff, 0xd8}):
		w, h, err := jpegSize(b)
		return "image/jpeg", w, h, err
	case len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		w, h, err := webpSize(b)
		return "image/webp", w, h, err
	case len(b) >= 12 && string(b[4:8]) == "ftyp" && isAVIF(b):
		w, h, err := avifSize(b)
		return "image/avif", w, h, err
	}
	return "", 0, 0, errors.New("is not AVIF, WebP, PNG or JPEG")
}

func jpegSize(b []byte) (int, int, error) {
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xff {
			return 0, 0, errors.New("is a JPEG with a broken marker")
		}
		marker := b[i+1]
		if marker == 0xff {
			i++
			continue
		}
		if marker == 0xd8 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			i += 2
			continue
		}
		size := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if size < 2 || i+2+size > len(b) {
			break
		}
		if marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc {
			if size < 7 {
				break
			}
			return int(binary.BigEndian.Uint16(b[i+7 : i+9])), int(binary.BigEndian.Uint16(b[i+5 : i+7])), nil
		}
		i += 2 + size
	}
	return 0, 0, errors.New("is a JPEG without a frame size")
}

func webpSize(b []byte) (int, int, error) {
	if len(b) < 30 {
		return 0, 0, errors.New("is a short WebP")
	}
	switch string(b[12:16]) {
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1, nil
	case "VP8L":
		if b[20] != 0x2f {
			return 0, 0, errors.New("is a WebP with a bad lossless signature")
		}
		bits := binary.LittleEndian.Uint32(b[21:25])
		return int(bits&0x3fff) + 1, int(bits>>14&0x3fff) + 1, nil
	case "VP8 ":
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, errors.New("is a WebP with a bad lossy start code")
		}
		return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff), nil
	}
	return 0, 0, errors.New("is a WebP without a known image chunk")
}

type box struct {
	kind string
	body []byte
}

// boxes splits ISO-BMFF boxes; a box that runs past its parent is refused.
func boxes(b []byte) ([]box, error) {
	var out []box
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, errors.New("is an AVIF with a truncated box")
		}
		size := uint64(binary.BigEndian.Uint32(b[0:4]))
		head := uint64(8)
		switch size {
		case 0:
			size = uint64(len(b))
		case 1:
			if len(b) < 16 {
				return nil, errors.New("is an AVIF with a truncated box")
			}
			size, head = binary.BigEndian.Uint64(b[8:16]), 16
		}
		if size < head || size > uint64(len(b)) {
			return nil, errors.New("is an AVIF with a box past its end")
		}
		out = append(out, box{string(b[4:8]), b[head:size]})
		b = b[size:]
	}
	return out, nil
}

func isAVIF(b []byte) bool {
	size := int(binary.BigEndian.Uint32(b[0:4]))
	if size < 16 || size > len(b) {
		return false
	}
	for i := 8; i+4 <= size; i += 4 {
		if i == 12 { // minor version
			continue
		}
		if brand := string(b[i : i+4]); brand == "avif" || brand == "avis" {
			return true
		}
	}
	return false
}

// avifSize reads every ispe under meta/iprp/ipco: each one is bounded, and the
// first, the primary item's in practice, gives the size.
func avifSize(b []byte) (int, int, error) {
	top, err := boxes(b)
	if err != nil {
		return 0, 0, err
	}
	find := func(list []box, kind string) []byte {
		for _, x := range list {
			if x.kind == kind {
				return x.body
			}
		}
		return nil
	}
	meta := find(top, "meta")
	if len(meta) < 4 {
		return 0, 0, errors.New("is an AVIF without meta")
	}
	inMeta, err := boxes(meta[4:])
	if err != nil {
		return 0, 0, err
	}
	iprp := find(inMeta, "iprp")
	inIprp, err := boxes(iprp)
	if err != nil {
		return 0, 0, err
	}
	props, err := boxes(find(inIprp, "ipco"))
	if err != nil {
		return 0, 0, err
	}
	w, h := 0, 0
	for _, p := range props {
		if p.kind != "ispe" {
			continue
		}
		if len(p.body) < 12 {
			return 0, 0, errors.New("is an AVIF with a short ispe")
		}
		pw, ph := int(binary.BigEndian.Uint32(p.body[4:8])), int(binary.BigEndian.Uint32(p.body[8:12]))
		if pw > ImageMaxPixels || ph > ImageMaxPixels {
			return pw, ph, nil
		}
		if w == 0 {
			w, h = pw, ph
		}
	}
	if w == 0 {
		return 0, 0, errors.New("is an AVIF without ispe")
	}
	return w, h, nil
}
