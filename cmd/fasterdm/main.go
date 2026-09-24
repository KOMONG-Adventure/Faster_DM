package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
	"os"
	"os/signal"
)

func main() {
	url := flag.String("url", "", "татах HTTP/HTTPS холбоос")
	out := flag.String("out", "", "хадгалах шинэ файлын зам")
	workers := flag.Int("workers", 16, "зэрэгцээ worker-ийн тоо (1–128)")
	flag.Parse()
	if *url == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	cfg := download.DefaultConfig()
	cfg.Workers = *workers
	engine, err := download.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	updates := make(chan download.Snapshot, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for s := range updates {
			fmt.Fprintf(os.Stderr, "\rТатсан: %.2f MiB | %.2f MiB/s | хэсэг: %d | %s    ", float64(s.Downloaded)/(1<<20), s.BytesPerSecond/(1<<20), len(s.Chunks), s.Status)
		}
	}()
	result, err := engine.Download(ctx, *url, *out, updates)
	close(updates)
	<-done
	cancel()
	engine.Close()
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Алдаа:", err)
		os.Exit(1)
	}
	fmt.Printf("Дууслаа: %s (%d байт, %s)\n", result.Path, result.Bytes, result.Duration.Round(1e6))
}
