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
	"time"

	"github.com/golang/glog"
)

// QSM client without authentication
type Client struct {
	apiKey     string
	baseURL    string
	HTTPClient *http.Client
}

// ClientOptions are options for QSM http client.
type ClientOptions struct {
	Https      bool
	ReqTimeout time.Duration
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
func NewClient(ip string, opts ClientOptions) *Client {
	client := &Client{
		HTTPClient: &http.Client{},
		baseURL:    "http://" + ip,
	}

	if opts.ReqTimeout != 0 {
		client.HTTPClient.Timeout = opts.ReqTimeout
	}

	if opts.Https {
	}

	return client
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

func (c *AuthClient) SendRequest(ctx context.Context, req *http.Request, v interface{}) error {
	res, err := c.doSendRequest(ctx, req, v)
	if err != nil {
		return err
	}

	if res.StatusCode == 401 {
		res.Body.Close()

		// When the existing access token expired, generate a new access token.
		glog.V(2).Infof("[AuthSendRequest] generate new access token. (%s%s)\n", req.Host, req.URL.Path)
		authRes, err := c.genAccessToken(ctx, c.refreshToken)
		if err != nil {
			return fmt.Errorf("genAccessToken failed: %v\n", err)
		}

		// Update new access token then send request again
		c.accessToken = authRes.AccessToken
		c.apiKey = authRes.AccessToken
		glog.V(2).Infof("[AuthSendRequest] SendRequest again (%s%s)\n", req.Host, req.URL.Path)
		res, err = c.doSendRequest(ctx, req, v)
	}

	defer res.Body.Close()

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

	return err

}

func (c *Client) SendRequest(ctx context.Context, req *http.Request, v interface{}) error {
	res, err := c.doSendRequest(ctx, req, v)
	if err != nil {
		return err
	}

	defer res.Body.Close()

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

	return err

}

func (c *Client) doSendRequest(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.apiKey != "" {
		glog.V(5).Infof("[doSendRequest] apiKey: %s\n", c.apiKey)
		req.Header.Set("Authorization", c.apiKey)
	}

	req = req.WithContext(ctx)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		glog.Errorf("[doSendRequest] err: %v\n", err)
		return nil, err
	}

	glog.V(4).Infof("[doSendRequest] StatusCode: %d (%s%s)\n", res.StatusCode, req.Host, req.URL.Path)
	return res, nil
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
	if err := c.SendRequest(ctx, req, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// Generate a new access token from refresh token
func (c *Client) genAccessToken(ctx context.Context, t string) (*AuthRes, error) {
	params := url.Values{}
	params.Add("refreshToken", t)

	req, err := c.NewRequest(ctx, http.MethodPost, "/auth/refresh", params)
	if err != nil {
		return nil, err
	}

	res := AuthRes{}
	if err := c.SendRequest(ctx, req, &res); err != nil {
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

	return &AuthClient{
		Client: Client{
			apiKey:     res.AccessToken,
			baseURL:    c.baseURL,
			HTTPClient: c.HTTPClient,
		},
		accessToken:  res.AccessToken,
		refreshToken: res.RefreshToken,
	}, nil
}
