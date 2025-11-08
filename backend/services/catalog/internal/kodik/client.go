package kodik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://kodikapi.com/list"

type Client struct {
	token  string
	client *http.Client
}

type Translation struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type Material struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Link          string                 `json:"link"`
	Title         string                 `json:"title"`
	TitleOrig     string                 `json:"title_orig"`
	OtherTitle    string                 `json:"other_title"`
	Description   string                 `json:"description"`
	Year          int                    `json:"year"`
	EpisodesCount int                    `json:"episodes_count"`
	PosterURL     string                 `json:"poster_url"`
	Image         string                 `json:"image"`
	KinopoiskID   string                 `json:"kinopoisk_id"`
	ImdbID        string                 `json:"imdb_id"`
	ShikimoriID   string                 `json:"shikimori_id"`
	Translation   *Translation           `json:"translation"`
	Raw           map[string]interface{} `json:"-"`
}

type ListResponse struct {
	Time     string     `json:"time"`
	Total    int        `json:"total"`
	PrevPage *string    `json:"prev_page"`
	NextPage *string    `json:"next_page"`
	Results  []Material `json:"results"`
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}


func (c *Client) FetchPage(ctx context.Context, limit int, next string, types string, withEpisodes bool, withMaterialData bool) (*ListResponse, error) {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("token", c.token)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	if next != "" {
		q.Set("next", next)
	}
	if types != "" {
		q.Set("types", types)
	}
	if withEpisodes {
		q.Set("with_episodes", "true")
	}
	if withMaterialData {
		q.Set("with_material_data", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var lr ListResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&lr); err != nil {
		return nil, err
	}

	for i := range lr.Results {
		if lr.Results[i].PosterURL == "" && lr.Results[i].Image != "" {
			lr.Results[i].PosterURL = lr.Results[i].Image
		}
	}
	return &lr, nil
}

func (c *Client) FetchByID(ctx context.Context, id string, withMaterialData bool) (*Material, error) {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("token", c.token)
	q.Set("id", id)
	q.Set("limit", "1")
	if withMaterialData {
		q.Set("with_material_data", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var lr ListResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&lr); err != nil {
		return nil, err
	}

	if len(lr.Results) == 0 {
		return nil, fmt.Errorf("material with id %s not found", id)
	}

	m := lr.Results[0]
	if m.PosterURL == "" && m.Image != "" {
		m.PosterURL = m.Image
	}
	return &m, nil
}

func (c *Client) Search(ctx context.Context, title string, limit int, withMaterialData bool) (*ListResponse, error) {
	u, _ := url.Parse("https://kodikapi.com/search")
	q := u.Query()
	q.Set("token", c.token)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	if title != "" {
		q.Set("title", title)
	}
	if withMaterialData {
		q.Set("with_material_data", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var lr ListResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&lr); err != nil {
		return nil, err
	}

	for i := range lr.Results {
		if lr.Results[i].PosterURL == "" && lr.Results[i].Image != "" {
			lr.Results[i].PosterURL = lr.Results[i].Image
		}
	}
	return &lr, nil
}

func (c *Client) SearchByKinopoiskID(ctx context.Context, kinopoiskID string, limit int, withMaterialData bool) (*ListResponse, error) {
	u, _ := url.Parse("https://kodikapi.com/search")
	q := u.Query()
	q.Set("token", c.token)
	q.Set("kinopoisk_id", kinopoiskID)
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	if withMaterialData {
		q.Set("with_material_data", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var lr ListResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&lr); err != nil {
		return nil, err
	}
	for i := range lr.Results {
		if lr.Results[i].PosterURL == "" && lr.Results[i].Image != "" {
			lr.Results[i].PosterURL = lr.Results[i].Image
		}
	}
	return &lr, nil
}

