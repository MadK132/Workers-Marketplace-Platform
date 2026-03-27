package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type UserClient struct {
	baseURL string
	client  *http.Client
}

func NewUserClient(baseURL string) *UserClient {
	return &UserClient{baseURL: baseURL, client: &http.Client{}}
}

type CustomerProfileResponse struct {
	CustomerProfileID int `json:"customer_profile_id"`
}

func (c *UserClient) GetCustomerProfile(
	ctx context.Context,
	userID int,
	token string,
) (int, error) {

	url := fmt.Sprintf("%s/api/internal/customer-profile?user_id=%d", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("user-service error: %d", resp.StatusCode)
	}

	var result CustomerProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.CustomerProfileID, nil
}

type WorkerProfileResponse struct {
	WorkerProfileID int `json:"worker_profile_id"`
}

func (c *UserClient) GetWorkerProfile(
	ctx context.Context,
	userID int,
	token string,
) (int, error) {

	url := fmt.Sprintf("%s/api/internal/worker-profile?user_id=%d", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("user-service error")
	}

	var result WorkerProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.WorkerProfileID, nil
}
