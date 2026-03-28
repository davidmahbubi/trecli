package trello

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const BaseURL = "https://api.trello.com/1"

type Client struct {
	apiKey   string
	apiToken string
	http     *http.Client
}

type Board struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type List struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Pos  float64 `json:"pos"`
}

type Card struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Desc   string  `json:"desc"`
	IDList string  `json:"idList"`
	Pos    float64 `json:"pos"`
}

func NewClient(key, token string) *Client {
	return &Client{
		apiKey:   key,
		apiToken: token,
		http:     &http.Client{},
	}
}

func (c *Client) do(method, path string, query map[string]string, body []byte) ([]byte, error) {
	u, err := url.Parse(BaseURL + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("key", c.apiKey)
	q.Set("token", c.apiToken)
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) GetBoards() ([]Board, error) {
	data, err := c.do("GET", "/members/me/boards", map[string]string{"fields": "name,desc"}, nil)
	if err != nil {
		return nil, err
	}
	var boards []Board
	if err := json.Unmarshal(data, &boards); err != nil {
		return nil, err
	}
	return boards, nil
}

func (c *Client) GetLists(boardID string) ([]List, error) {
	path := fmt.Sprintf("/boards/%s/lists", boardID)
	data, err := c.do("GET", path, map[string]string{"fields": "name,pos"}, nil)
	if err != nil {
		return nil, err
	}
	var lists []List
	if err := json.Unmarshal(data, &lists); err != nil {
		return nil, err
	}
	return lists, nil
}

func (c *Client) GetCardsInList(listID string) ([]Card, error) {
	path := fmt.Sprintf("/lists/%s/cards", listID)
	data, err := c.do("GET", path, map[string]string{"fields": "name,desc,idList,pos"}, nil)
	if err != nil {
		return nil, err
	}
	var cards []Card
	if err := json.Unmarshal(data, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func (c *Client) CreateCard(listID, name, desc string) (*Card, error) {
	query := map[string]string{
		"idList": listID,
		"name":   name,
		"desc":   desc,
	}
	data, err := c.do("POST", "/cards", query, nil)
	if err != nil {
		return nil, err
	}
	var card Card
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

func (c *Client) UpdateCardList(cardID, newListID string) error {
	query := map[string]string{
		"idList": newListID,
	}
	path := fmt.Sprintf("/cards/%s", cardID)
	_, err := c.do("PUT", path, query, nil)
	return err
}

func (c *Client) ArchiveCard(cardID string) error {
	query := map[string]string{
		"closed": "true",
	}
	path := fmt.Sprintf("/cards/%s", cardID)
	_, err := c.do("PUT", path, query, nil)
	return err
}
