package activity

import (
	"context"
	"parental-control/internal/lib/types"
	"sync"
	"time"
)

const flushInterval = 10 * time.Second

type counters struct {
	keyboard uint32
	mouse    uint32
}

func counterDelta(previous, current uint32) uint32 { return current - previous }

func classify(previous, current counters) types.ActivityKind {
	keyboard := counterDelta(previous.keyboard, current.keyboard) > 0
	mouse := counterDelta(previous.mouse, current.mouse) > 0
	switch {
	case keyboard && mouse:
		return types.ActivityBoth
	case keyboard:
		return types.ActivityKeyboard
	case mouse:
		return types.ActivityMouse
	default:
		return types.ActivityNone
	}
}

// Track samples cumulative HID counters once per second. It records only whether
// keyboard and/or mouse activity happened, never event contents or coordinates.
func Track(ctx context.Context, commands chan<- types.AppCommand) {
	defer ctx.Value(types.WgKey{}).(*sync.WaitGroup).Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	var previous counters
	haveBaseline := false
	batch := make([]types.ActivitySample, 0, 10)
	var lastBucket time.Time
	flush := func() {
		if len(batch) == 0 {
			return
		}
		out := append([]types.ActivitySample(nil), batch...)
		select {
		case commands <- types.ActivityBatch{Samples: out}:
			batch = batch[:0]
		case <-ctx.Done():
		}
	}

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				commands <- types.ActivityBatch{Samples: append([]types.ActivitySample(nil), batch...)}
			}
			return
		case <-flushTicker.C:
			flush()
		case now := <-ticker.C:
			if !PreflightAccess() {
				haveBaseline = false
				continue
			}
			current := readCounters()
			if !haveBaseline {
				previous, haveBaseline = current, true
				continue
			}
			kind := classify(previous, current)
			previous = current
			bucket := now.Truncate(5 * time.Minute)
			if !lastBucket.IsZero() && !bucket.Equal(lastBucket) {
				flush()
			}
			lastBucket = bucket
			if kind != types.ActivityNone {
				batch = append(batch, types.ActivitySample{At: now, Kind: kind})
			}
		}
	}
}
