// Package browserbridge exchanges bounded native messages and local link drafts.
package browserbridge

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const HostName = "com.komong.fasterdm"

// ExtensionID is generated from the public manifest key (not a secret).
const ExtensionID = "ppifjgnggblnhpndbldhbcakkbcipkof"
const MaxMessage = 64 << 10

type Message struct {
	URL string `json:"url"`
}
type Reply struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
type Draft struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func Validate(raw string) error {
	u, err := url.Parse(raw)
	if len(raw) > 16384 || raw == "" || strings.IndexFunc(raw, unicode.IsSpace) >= 0 || err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("Зөвхөн HTTP/HTTPS холбоос дамжуулна")
	}
	return nil
}
func Read(r io.Reader) (Message, error) {
	var m Message
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return m, err
	}
	if size == 0 || size > MaxMessage {
		return m, errors.New("хүсэлт хэт том")
	}
	b := make([]byte, size)
	if _, err := io.ReadFull(r, b); err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, Validate(m.URL)
}
func Write(w io.Writer, r Reply) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err = binary.Write(w, binary.LittleEndian, uint32(len(b))); err != nil {
		return err
	}
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}
func Directory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "FasterDM", "browser-inbox"), nil
}
func Queue(dir, raw string) (string, error) {
	if err := Validate(raw); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(files) >= 64 {
		return "", errors.New("Аппд хүлээгдэж буй холбоосуудыг эхлээд оруулна уу")
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(random[:])
	b, _ := json.Marshal(Draft{ID: id, URL: raw})
	f, err := os.CreateTemp(dir, ".draft-")
	if err != nil {
		return "", err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if err = f.Chmod(0600); err != nil {
		return "", err
	}
	if _, err = f.Write(b); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return id, os.Rename(f.Name(), filepath.Join(dir, id+".json"))
}
func validID(id string) bool { _, err := hex.DecodeString(id); return len(id) == 32 && err == nil }
func Peek(dir string) (Draft, error) {
	files, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return Draft{}, nil
	}
	if err != nil {
		return Draft{}, err
	}
	for _, entry := range files {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validID(id) {
			continue
		}
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			return Draft{}, err
		}
		b, err := io.ReadAll(io.LimitReader(f, MaxMessage+1))
		f.Close()
		if err != nil {
			return Draft{}, err
		}
		var d Draft
		if len(b) > MaxMessage || json.Unmarshal(b, &d) != nil || d.ID != id || Validate(d.URL) != nil {
			return Draft{}, errors.New("Браузерын холбоосын мэдээлэл гэмтсэн")
		}
		return d, nil
	}
	return Draft{}, nil
}
func Ack(dir, id string) error {
	if !validID(id) {
		return errors.New("Буруу хүсэлтийн ID")
	}
	err := os.Remove(filepath.Join(dir, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
