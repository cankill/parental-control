package bot

import (
	"context"
	"fmt"
	"parental-control/internal/helper"
	"sync"
)

var youtubeDomains = []string{"youtube.com", "www.youtube.com"}

type youtubeTimer struct {
	mu         sync.Mutex
	parent     context.Context
	ctx        context.Context
	cancelFunc context.CancelFunc
	client     *helper.Client
}

func newYoutubeTimer(parent context.Context) *youtubeTimer {
	ctx, cancel := context.WithCancel(parent)
	return &youtubeTimer{parent: parent, ctx: ctx, cancelFunc: cancel, client: helper.NewClient()}
}

func (t *youtubeTimer) reset() context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
	t.ctx, t.cancelFunc = context.WithCancel(t.parent)
	return t.ctx
}

func (t *youtubeTimer) cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
}

func (t *youtubeTimer) block() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.client.BlockDomains(youtubeDomains); err != nil {
		fmt.Printf("Failed to block youtube via helper: %s\n", err)
	}
}

func (t *youtubeTimer) unblock() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.client.UnblockDomains(youtubeDomains); err != nil {
		fmt.Printf("Failed to unblock youtube via helper: %s\n", err)
	}
}
