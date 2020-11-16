package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
)

const (
	ErrNotFound   = Error("Not Found")
	ErrUnexpected = Error("Unexpected response")
)

type Error string

func (e Error) Error() string { return string(e) }

type connectClient struct {
	baseURL *url.URL
}

func NewConnectClient(baseUrl string) (*connectClient, error) {

	url, err := url.Parse(baseUrl)
	return &connectClient{baseURL: url}, err
}

func unmarshalResult(body []byte, inerr error) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error unmarshalling response, err: %v, body: '%s'", err, string(body)))
	}
	return result, inerr
}

func (c *connectClient) GetConfig(name string) (map[string]interface{}, error) {
	url, err := c.baseURL.Parse(path.Join("connectors", name, "config"))
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		return unmarshalResult(body, nil)
	}

	if resp.StatusCode == http.StatusNotFound {
		return unmarshalResult(body, ErrNotFound)
	}

	return nil, fmt.Errorf("%s code %d with body %s", ErrUnexpected, resp.StatusCode, string(body))
}

func (c *connectClient) PutConfig(name string, config interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	url, err := c.baseURL.Parse(path.Join("connectors", name, "config"))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, url.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return unmarshalResult(body, nil)
	}

	if resp.StatusCode == http.StatusNotFound {
		return unmarshalResult(body, ErrNotFound)
	}

	return nil, fmt.Errorf("%s code %d with body %s", ErrUnexpected, resp.StatusCode, string(body))
}

func (c *connectClient) Delete(name string) error {
	url, err := c.baseURL.Parse(path.Join("connectors", name))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return fmt.Errorf("%s code %d with body %s", ErrUnexpected, resp.StatusCode, string(body))
}
