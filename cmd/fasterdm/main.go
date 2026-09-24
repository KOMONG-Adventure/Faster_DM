package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
)

func main() {
	url := flag.String("url", "", "татах HTTP/HTTPS холбоос")
	out := flag.String("out", "", "хадгалах шинэ файлын зам")
	workers := flag.Int("workers", 16, "зэрэгцээ worker-ийн тоо (1–128)")
	flag.Parse()
	// Explorer-оос давхар товшсон үед аргумент ирдэггүй тул асуулттай горим нээнэ.
	interactive := len(os.Args) == 1
	input := bufio.NewScanner(os.Stdin)
	exitCode := 0
	if interactive {
		fmt.Println("Faster DM — файл татах")
		fmt.Println("Татах холбоосоо оруулаад Enter дарна уу.")
		fmt.Println("Энэ нь текстэн цонхтой хувилбар; график интерфэйс хараахан нэмэгдээгүй.")
		fmt.Print("\nТатах холбоос: ")
		if input.Scan() {
			*url = strings.TrimSpace(input.Text())
		}
		if *url != "" {
			folder, err := os.UserHomeDir()
			if err != nil {
				folder, _ = os.Getwd()
			}
			if info, err := os.Stat(filepath.Join(folder, "Downloads")); err == nil && info.IsDir() {
				folder = filepath.Join(folder, "Downloads")
			}
			defaultPath := filepath.Join(folder, "download.bin")
			fmt.Printf("Хадгалах файлын бүтэн зам (Enter = %s): ", defaultPath)
			if input.Scan() {
				*out = strings.Trim(strings.TrimSpace(input.Text()), "\"")
			}
			if *out == "" {
				*out = defaultPath
			}
		}
	}
	if *url == "" || *out == "" {
		if interactive {
			fmt.Fprintln(os.Stderr, "Татах холбоос оруулаагүй тул таталт эхлээгүй.")
		} else {
			flag.Usage()
		}
		exitCode = 2
	} else if err := downloadFile(*url, *out, *workers); err != nil {
		fmt.Fprintln(os.Stderr, "Алдаа:", err)
		exitCode = 1
	}
	if interactive {
		fmt.Print("\nХаахын тулд Enter дарна уу...")
		input.Scan()
	}
	os.Exit(exitCode)
}

func downloadFile(url, out string, workers int) error {
	fmt.Printf("\nХадгалах файл: %s\nТаталтыг зогсоох: Ctrl+C\n", out)
	cfg := download.DefaultConfig()
	cfg.Workers = workers
	engine, err := download.New(cfg)
	if err != nil {
		return err
	}
	defer engine.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	updates := make(chan download.Snapshot, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for s := range updates {
			fmt.Fprintf(os.Stderr, "\rТатсан: %.2f MiB | %.2f MiB/s | хэсэг: %d | %s    ", float64(s.Downloaded)/(1<<20), s.BytesPerSecond/(1<<20), len(s.Chunks), s.Status)
		}
	}()
	result, err := engine.Download(ctx, url, out, updates)
	close(updates)
	<-done
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	fmt.Printf("Дууслаа: %s (%d байт, %s)\n", result.Path, result.Bytes, result.Duration.Round(1e6))
	return nil
}
