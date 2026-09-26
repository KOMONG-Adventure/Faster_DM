package main

import "github.com/KOMONG-Adventure/Faster_DM/internal/browserbridge"

func (a *App) GetBrowserLink() (browserbridge.Draft, error) {
	dir, err := browserbridge.Directory()
	if err != nil {
		return browserbridge.Draft{}, err
	}
	return browserbridge.Peek(dir)
}
func (a *App) AcceptBrowserLink(id string) error {
	dir, err := browserbridge.Directory()
	if err != nil {
		return err
	}
	return browserbridge.Ack(dir, id)
}
