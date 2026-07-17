package bot

import (
	"context"
	"fmt"
	"parental-control/internal/lib/types"
)

type statisticsClient struct {
	ctx      context.Context
	requests chan<- types.AppCommand
}

func newStatisticsClient(ctx context.Context, requests chan<- types.AppCommand) *statisticsClient {
	return &statisticsClient{ctx: ctx, requests: requests}
}

func (c *statisticsClient) hourly(shift int) (*types.AppInfoResponse, error) {
	response := make(chan *types.AppInfoResponse, 1)
	if err := c.send(types.RequestCommand{ResponseChan: response, ShiftHours: shift}); err != nil {
		return nil, err
	}
	return receive(c.ctx, response)
}

func (c *statisticsClient) daily(shift int) (*types.AppInfoResponse, error) {
	response := make(chan *types.AppInfoResponse, 1)
	if err := c.send(types.DayRequest{DayShift: shift, ResponseChan: response}); err != nil {
		return nil, err
	}
	return receive(c.ctx, response)
}

func (c *statisticsClient) sites(shift int) (*types.AppInfoResponse, error) {
	response := make(chan *types.AppInfoResponse, 1)
	if err := c.send(types.DomainRequest{ShiftHours: shift, ResponseChan: response}); err != nil {
		return nil, err
	}
	return receive(c.ctx, response)
}

func (c *statisticsClient) appInfo(name string) (string, error) {
	response := make(chan string, 1)
	if err := c.send(types.AppInfoQuery{Name: name, ResponseChan: response}); err != nil {
		return "", err
	}
	return receive(c.ctx, response)
}

func (c *statisticsClient) activity(shift int) (*types.ActivityResponse, error) {
	response := make(chan *types.ActivityResponse, 1)
	if err := c.send(types.ActivityRequest{ShiftHours: shift, ResponseChan: response}); err != nil {
		return nil, err
	}
	return receive(c.ctx, response)
}

func (c *statisticsClient) send(command types.AppCommand) error {
	select {
	case c.requests <- command:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("shutting down")
	}
}

func receive[T any](ctx context.Context, response <-chan T) (T, error) {
	select {
	case value := <-response:
		return value, nil
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("shutting down")
	}
}
