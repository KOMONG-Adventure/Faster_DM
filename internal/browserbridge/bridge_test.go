package browserbridge

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestFramesAndURLValidation(t *testing.T) {
	for _, raw := range []string{"file:///secret", "https://user:pass@example.com/x", "javascript:alert(1)", "https://example.com/a b"} {
		if Validate(raw) == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	body := []byte(`{"url":"https://example.com/file.zip"}`)
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(len(body)))
	b.Write(body)
	if m, err := Read(&b); err != nil || m.URL != "https://example.com/file.zip" {
		t.Fatalf("%+v %v", m, err)
	}
	for _, size := range []uint32{0, MaxMessage + 1, 10} {
		b.Reset()
		binary.Write(&b, binary.LittleEndian, size)
		if _, err := Read(&b); err == nil {
			t.Fatal("accepted invalid frame")
		}
	}
}

func TestInbox(t *testing.T) {
	dir := t.TempDir()
	id, err := Queue(dir, "https://example.com/file.zip")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Peek(dir)
	if err != nil || d.ID != id {
		t.Fatalf("%+v %v", d, err)
	}
	if err := Ack(dir, "../outside"); err == nil {
		t.Fatal("accepted traversal")
	}
	if err := Ack(dir, id); err != nil {
		t.Fatal(err)
	}
	d, err = Peek(dir)
	if err != nil || d.ID != "" {
		t.Fatalf("%+v %v", d, err)
	}
}
