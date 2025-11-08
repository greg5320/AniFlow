package kodik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"strconv"
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
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Link           string                 `json:"link"`
	Title          string                 `json:"title"`
	TitleOrig      string                 `json:"title_orig"`
	OtherTitle     string                 `json:"other_title"`
	Description    string                 `json:"description"`
	Year           int                    `json:"year"`
	EpisodesCount  int                    `json:"episodes_count"`
	PosterURL      string                 `json:"poster_url"`
	Image          string                 `json:"image"`
	AnimePosterURL string                 `json:"anime_poster_url"` 
	Genres         []string               `json:"genres"`
	KinopoiskID    string                 `json:"kinopoisk_id"`
	KinopoiskRating float64               `json:"kinopoisk_rating"`
	Translation    *Translation           `json:"translation"`
	Raw            map[string]interface{} `json:"-"`
}

type ListResponse struct {
	Time     string     `json:"time"`
	Total    int        `json:"total"`
	PrevPage *string    `json:"prev_page"`
	NextPage *string    `json:"next_page"`
	Results  []Material `json:"results"`
}

func toStr(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		return fmt.Sprintf("%v", t)
	}
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
	doRequest := func(u string) ([]map[string]interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var raw map[string]interface{}
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		resI, _ := raw["results"].([]interface{})
		out := make([]map[string]interface{}, 0, len(resI))
		for _, ri := range resI {
			if ri == nil {
				continue
			}
			if im, ok := ri.(map[string]interface{}); ok {
				out = append(out, im)
			}
		}
		return out, nil
	}

	u1, _ := url.Parse(baseURL)
	q1 := u1.Query()
	q1.Set("token", c.token)
	q1.Set("id", id)
	q1.Set("limit", "50")
	if withMaterialData {
		q1.Set("with_material_data", "true")
	}
	u1.RawQuery = q1.Encode()
	fmt.Printf("[kodik] FetchByID try /list URL=%s\n", u1.String())

	results, err := doRequest(u1.String())
	if err != nil {
		return nil, err
	}

	var found map[string]interface{}
	for _, im := range results {
		if toStr(im["id"]) == id {
			found = im
			break
		}
	}

	if found == nil {
		u2, _ := url.Parse("https://kodikapi.com/search")
		q2 := u2.Query()
		q2.Set("token", c.token)
		q2.Set("id", id)
		q2.Set("limit", "50")
		if withMaterialData {
			q2.Set("with_material_data", "true")
		}
		u2.RawQuery = q2.Encode()
		fmt.Printf("[kodik] FetchByID fallback /search URL=%s\n", u2.String())

		results2, err2 := doRequest(u2.String())
		if err2 != nil {
			return nil, fmt.Errorf("fetch by id failed (list not found, search fetch error: %v)", err2)
		}
		for _, im := range results2 {
			if toStr(im["id"]) == id {
				found = im
				break
			}
		}
		if found == nil {
			firstIDs := make([]string, 0, 6)
			for i, im := range results {
				if i >= 5 {
					break
				}
				firstIDs = append(firstIDs, toStr(im["id"]))
			}
			for i, im := range results2 {
				if i >= 5 {
					break
				}
				firstIDs = append(firstIDs, toStr(im["id"]))
			}
			return nil, fmt.Errorf("material with id %s not found in results (examples: %v)", id, firstIDs)
		}
	}

	itemMap := found
	m := &Material{Raw: itemMap}
	m.ID = toStr(itemMap["id"])
	m.Type = toStr(itemMap["type"])
	m.Link = toStr(itemMap["link"])
	m.Title = toStr(itemMap["title"])
	m.TitleOrig = toStr(itemMap["title_orig"])
	m.OtherTitle = toStr(itemMap["other_title"])
	m.Description = toStr(itemMap["description"])
	if yv, ok := itemMap["year"]; ok {
		switch t := yv.(type) {
		case float64:
			m.Year = int(t)
		case int:
			m.Year = t
		}
	}
	if ev, ok := itemMap["episodes_count"]; ok {
		switch t := ev.(type) {
		case float64:
			m.EpisodesCount = int(t)
		case int:
			m.EpisodesCount = t
		}
	}
	if v := itemMap["poster_url"]; v != nil && toStr(v) != "" {
		m.PosterURL = toStr(v)
	} else if v := itemMap["image"]; v != nil {
		m.PosterURL = toStr(v)
	}
	if g, ok := itemMap["genres"]; ok && g != nil {
		if arr, ok := g.([]interface{}); ok {
			for _, gi := range arr {
				m.Genres = append(m.Genres, toStr(gi))
			}
		}
	}
	if kp := itemMap["kinopoisk_id"]; kp != nil {
		m.KinopoiskID = toStr(kp)
	}
	if md, ok := itemMap["material_data"].(map[string]interface{}); ok {
		if p := md["poster_url"]; p != nil && toStr(p) != "" {
			m.AnimePosterURL = toStr(p)
		}
		if r := md["kinopoisk_rating"]; r != nil {
			switch t := r.(type) {
			case float64:
				m.KinopoiskRating = t
			case string:
				if v, err := strconv.ParseFloat(t, 64); err == nil {
					m.KinopoiskRating = v
				}
			}
		}
		if g2 := md["genres"]; g2 != nil {
			if arr, ok := g2.([]interface{}); ok && len(m.Genres) == 0 {
				for _, gi := range arr {
					m.Genres = append(m.Genres, toStr(gi))
				}
			}
		}
	}
	if tr, ok := itemMap["translation"].(map[string]interface{}); ok {
		idF := 0
		if idv := tr["id"]; idv != nil {
			switch t := idv.(type) {
			case float64:
				idF = int(t)
			case int:
				idF = t
			}
		}
		m.Translation = &Translation{
			ID:    idF,
			Title: toStr(tr["title"]),
			Type:  toStr(tr["type"]),
		}
	}
	if m.PosterURL == "" && m.AnimePosterURL != "" {
		m.PosterURL = m.AnimePosterURL
	}

	fmt.Printf("[kodik] FetchByID returning id=%s title=%s\n", m.ID, m.Title)
	return m, nil
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

	var raw map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	resultsI, _ := raw["results"].([]interface{})
	lr := &ListResponse{Results: make([]Material, 0)}
	for _, ri := range resultsI {
		if ri == nil {
			continue
		}
		if itemMap, ok := ri.(map[string]interface{}); ok {
			m := Material{Raw: itemMap}
			m.ID = toStr(itemMap["id"])
			m.Title = toStr(itemMap["title"])
			m.Description = toStr(itemMap["description"])
			if v := itemMap["poster_url"]; v != nil && toStr(v) != "" {
				m.PosterURL = toStr(v)
			} else if v := itemMap["image"]; v != nil {
				m.PosterURL = toStr(v)
			}
			if g, ok := itemMap["genres"]; ok && g != nil {
				if arr, ok := g.([]interface{}); ok {
					for _, gi := range arr {
						m.Genres = append(m.Genres, toStr(gi))
					}
				}
			}
			if kp := itemMap["kinopoisk_id"]; kp != nil {
				m.KinopoiskID = toStr(kp)
			}
			if md, ok := itemMap["material_data"].(map[string]interface{}); ok {
				if p := md["poster_url"]; p != nil && toStr(p) != "" {
					m.AnimePosterURL = toStr(p)
				}
				if r := md["kinopoisk_rating"]; r != nil {
					switch t := r.(type) {
					case float64:
						m.KinopoiskRating = t
					case string:
						if v, err := strconv.ParseFloat(t, 64); err == nil {
							m.KinopoiskRating = v
						}
					}
				}
				if g2 := md["genres"]; g2 != nil {
					if arr, ok := g2.([]interface{}); ok && len(m.Genres) == 0 {
						for _, gi := range arr {
							m.Genres = append(m.Genres, toStr(gi))
						}
					}
				}
			}
			if tr, ok := itemMap["translation"].(map[string]interface{}); ok {
				idF := 0
				if idv := tr["id"]; idv != nil {
					switch t := idv.(type) {
					case float64:
						idF = int(t)
					case int:
						idF = t
					}
				}
				m.Translation = &Translation{
					ID:    idF,
					Title: toStr(tr["title"]),
					Type:  toStr(tr["type"]),
				}
			}
			lr.Results = append(lr.Results, m)
		}
	}
	if t, ok := raw["total"]; ok {
		if tf, ok := t.(float64); ok {
			lr.Total = int(tf)
		}
	}
	return lr, nil
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

