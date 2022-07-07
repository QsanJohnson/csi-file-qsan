// @2022 QSAN Inc. All rights reserved

package goqsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
)

// QSM client without authentication
type Client struct {
	apiKey     string
	baseURL    string
	HTTPClient *http.Client
}

// QSM client with authentication
type AuthClient struct {
	Client
	accessToken  string
	refreshToken string
}

// For authentication
type AuthRes struct {
	AccessToken  string `json:"accessToken"`
	ExpireTime   int    `json:"expireTime"`
	RefreshToken string `json:"refreshToken"`
}

// Empty response data
type EmptyData []interface{}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// NewClient returns QSM client with given URL
func NewClient(ip string) *Client {
	return &Client{
		HTTPClient: &http.Client{},
		baseURL:    "http://" + ip,
	}
}

func (c *Client) NewRequest(ctx context.Context, method, urlPath string, body url.Values) (*http.Request, error) {
	var (
		req *http.Request
		err error
	)

	urlStr := c.baseURL + urlPath
	glog.V(2).Infof("[NewRequest] %s url: %s\n", method, urlStr)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	if body != nil {
		glog.V(3).Infof("[NewRequest] body: %v\n", body)
		req, err = http.NewRequest(method, u.String(), strings.NewReader(body.Encode()))
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}

	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *Client) sendRequest(ctx context.Context, req *http.Request, v interface{}) error {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.apiKey != "" {
		glog.V(4).Infof("[sendRequest] apiKey: %s\n", c.apiKey)
		req.Header.Set("Authorization", c.apiKey)
	}

	req = req.WithContext(ctx)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		glog.Errorf("[sendRequest] err: %v\n", err)
		return err
	}

	defer res.Body.Close()

	glog.V(2).Infof("[sendRequest] StatusCode: %d\n", res.StatusCode)
	if res.StatusCode != http.StatusOK {
		errRes := errorResponse{}
		if err = json.NewDecoder(res.Body).Decode(&errRes); err == nil {
			return errors.New(errRes.Error.Message)
		}

		return fmt.Errorf("unknown error, status code: %d", res.StatusCode)
	}

	if err = json.NewDecoder(res.Body).Decode(v); err != nil {
		return err
	}

	glog.V(4).Infof("[sendRequest] res: %+v\n", v)
	return nil
}

func (c *Client) login(ctx context.Context, user string, passwd string) (*AuthRes, error) {
	params := url.Values{}
	params.Add("user", user)
	params.Add("password", passwd)
	params.Add("offlineAccess", "true")

	req, err := c.NewRequest(ctx, http.MethodPost, "/auth/get", params)
	if err != nil {
		return nil, err
	}

	res := AuthRes{}
	if err := c.sendRequest(ctx, req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *Client) GetAuthClient(ctx context.Context, user string, passwd string) (*AuthClient, error) {
	res, err := c.login(ctx, user, passwd)
	if err != nil {
		return nil, fmt.Errorf("login failed: %v\n", err)
	}

	glog.V(3).Infof("AccessToken: %s\n", res.AccessToken)

	ret := &AuthClient{}
	ret.accessToken = res.AccessToken
	ret.refreshToken = res.RefreshToken
	ret.baseURL = c.baseURL
	ret.HTTPClient = c.HTTPClient
	ret.apiKey = res.AccessToken

	return ret, nil
}
