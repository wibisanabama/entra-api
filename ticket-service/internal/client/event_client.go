package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type EventClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewEventClient(baseURL string) *EventClient {
	return &EventClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *EventClient) ReserveTickets(ctx context.Context, ticketTypeID string, quantity int32) error {
	url := fmt.Sprintf("%s/api/v1/internal/tickets/%s/reserve", c.baseURL, ticketTypeID)
	return c.sendReservationRequest(ctx, url, quantity)
}

func (c *EventClient) ReleaseTickets(ctx context.Context, ticketTypeID string, quantity int32) error {
	url := fmt.Sprintf("%s/api/v1/internal/tickets/%s/release", c.baseURL, ticketTypeID)
	return c.sendReservationRequest(ctx, url, quantity)
}

func (c *EventClient) sendReservationRequest(ctx context.Context, url string, quantity int32) error {
	payload := map[string]int32{"quantity": quantity}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event-service returned status: %d", resp.StatusCode)
	}

	return nil
}
