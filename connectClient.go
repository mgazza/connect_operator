package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
)

const (
	ErrNotFound   = Error("Not Found")
	ErrUnexpected = Error("Unexpected response")
)

type Error string

func (e Error) Error() string { return string(e) }

type connectClient struct {
	baseURL string
}

func NewConnectClient(baseUrl string) *connectClient {
	return &connectClient{baseURL: baseUrl}
}

func unmarshalResult(body []byte, inerr error) (map[string]string, error) {
	result := map[string]string{}
	err := json.Unmarshal(body, result)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error unmarshalling response, err: %v, body: '%s'", err, string(body)))
	}
	return result, inerr
}

func (c *connectClient) GetConfig(name string) (map[string]string, error) {

	resp, err := http.Get(path.Join(c.baseURL, "connectors", name, "config"))
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

func (c *connectClient) PutConfig(name string, config interface{}) (map[string]string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, path.Join(c.baseURL, "connectors", name, "Config"), bytes.NewBuffer(data))
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

	if resp.StatusCode == http.StatusOK {
		return unmarshalResult(body, nil)
	}

	if resp.StatusCode == http.StatusNotFound {
		return unmarshalResult(body, ErrNotFound)
	}

	return nil, fmt.Errorf("%s code %d with body %s", ErrUnexpected, resp.StatusCode, string(body))
}

func (c *connectClient) Delete(name string) error {
	req, err := http.NewRequest(http.MethodDelete, path.Join(c.baseURL, "connectors", name), nil)
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
