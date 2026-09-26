package main

import (
	"github.com/KOMONG-Adventure/Faster_DM/internal/browserbridge"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	reply := browserbridge.Reply{}
	defer func() { _ = browserbridge.Write(os.Stdout, reply) }()
	if len(os.Args) < 2 || os.Args[1] != "chrome-extension://"+browserbridge.ExtensionID+"/" {
		reply.Message = "Өргөтгөлийн эх үүсвэр зөвшөөрөгдөөгүй"
		return
	}
	m, err := browserbridge.Read(os.Stdin)
	if err != nil {
		reply.Message = err.Error()
		return
	}
	dir, err := browserbridge.Directory()
	if err != nil {
		reply.Message = err.Error()
		return
	}
	exe, err := os.Executable()
	if err != nil {
		reply.Message = err.Error()
		return
	}
	id, err := browserbridge.Queue(dir, m.URL)
	if err != nil {
		reply.Message = err.Error()
		return
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(exe), "FasterDM.exe"), "--browser-link")
	if err = cmd.Start(); err != nil {
		_ = browserbridge.Ack(dir, id)
		reply.Message = "Faster DM нээж чадсангүй. Installer-ийг дахин суулгана уу."
		return
	}
	_ = cmd.Process.Release()
	reply.OK = true
	reply.Message = "Холбоос дамжлаа. Faster DM дээр зайг шалгаад татаж эхэлнэ үү."
}
