// Package download нь HTTP файлыг зэрэгцээ татаж, нэг файлд шууд бичнэ.
package download

import (
	"errors"
	"net/http"
	"time"
)

var (
	ErrProtocol = errors.New("HTTP range протоколын зөрчил")
	ErrChanged  = errors.New("серверийн файл өөрчлөгдсөн")
)

type Config struct {
	Workers          int
	MinChunkSize     int64
	BufferSize       int
	MaxRetries       int
	IdleTimeout      time.Duration
	ProgressInterval time.Duration
	// Client-ийг өгвөл engine түүнийг өөрчлөхгүй. Timeout нь хүсэлтийн нийт хугацааг хязгаарлана.
	Client *http.Client
}

func DefaultConfig() Config {
	return Config{Workers: 16, MinChunkSize: 1 << 20, BufferSize: 128 << 10,
		MaxRetries: 5, IdleTimeout: 30 * time.Second, ProgressInterval: time.Second / 30}
}

type ChunkSnapshot struct {
	ID             int     `json:"id"`
	Start          int64   `json:"start"`
	End            int64   `json:"end"` // Хамаарахгүй төгсгөл; тодорхойгүй бол -1.
	Downloaded     int64   `json:"downloaded"`
	BytesPerSecond float64 `json:"bytesPerSecond"`
	Status         string  `json:"status"`
	Retries        int     `json:"retries"`
}

type Snapshot struct {
	Total          int64           `json:"total"`
	Downloaded     int64           `json:"downloaded"`
	BytesPerSecond float64         `json:"bytesPerSecond"`
	Status         string          `json:"status"`
	Chunks         []ChunkSnapshot `json:"chunks"`
}

type Result struct {
	Path     string
	Bytes    int64
	Parallel bool
	Duration time.Duration
}
