// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	gopath "path"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/httpbakery"
	"github.com/juju/errors"
	"gopkg.in/httprequest.v1"

	"github.com/juju/juju/rpc/params"
	jujuversion "github.com/juju/juju/version"
)

// HTTPClient implements Connection.APICaller.HTTPClient and returns an HTTP
// client pointing to the API server "/model/:uuid/" path.
func (s *state) HTTPClient() (*httprequest.Client, error) {
	apiPath, err := apiPath(s.modelTag.Id(), "/")
	if err != nil {
		return nil, errors.Trace(err)
	}
	url := s.Addr()
	url.Scheme = s.serverScheme
	url.Path = gopath.Join(url.Path, apiPath)

	return s.httpClient(url)
}

// RootHTTPClient implements Connection.APICaller.HTTPClient and returns an HTTP
// client pointing to the API server root path.
func (s *state) RootHTTPClient() (*httprequest.Client, error) {
	url := s.Addr()
	url.Scheme = s.serverScheme
	return s.httpClient(url)
}

func (s *state) httpClient(baseURL *url.URL) (*httprequest.Client, error) {
	if !s.isLoggedIn() {
		return nil, errors.New("no HTTP client available without logging in")
	}
	return &httprequest.Client{
		BaseURL: baseURL.String(),
		Doer: httpRequestDoer{
			st: s,
		},
		UnmarshalError: unmarshalHTTPErrorResponse,
	}, nil
}

// httpRequestDoer implements httprequest.Doer and httprequest.DoerWithBody
// by using httpbakery and the state to make authenticated requests to
// the API server.
type httpRequestDoer struct {
	st *state
}

var _ httprequest.Doer = httpRequestDoer{}

// Do implements httprequest.Doer.Do.
func (doer httpRequestDoer) Do(req *http.Request) (*http.Response, error) {
	if err := authHTTPRequest(
		req,
		doer.st.loginProvider,
	); err != nil {
		return nil, errors.Trace(err)
	}
	return doer.st.bakeryClient.DoWithCustomError(req, func(resp *http.Response) error {
		// At this point we are only interested in errors that
		// the bakery cares about, and the CodeDischargeRequired
		// error is the only one, and that always comes with a
		// response code StatusUnauthorized.
		if resp.StatusCode != http.StatusUnauthorized {
			return nil
		}
		return bakeryError(unmarshalHTTPErrorResponse(resp))
	})
}

// AuthHTTPRequest adds Juju auth info (username, password, nonce, macaroons)
// to the given HTTP request, suitable for sending to a Juju API server.
func AuthHTTPRequest(req *http.Request, info *Info) error {
	lp := NewLegacyLoginProvider(info.Tag, info.Password, info.Nonce, info.Macaroons, nil, nil)
	return authHTTPRequest(req, lp)
}

func authHTTPRequest(req *http.Request, lp LoginProvider) error {
	header, err := lp.AuthHeader()
	if err != nil {
		return errors.Trace(err)
	}
	// Copy headers to the request, using the first available value for each key.
	for key := range header {
		req.Header.Set(key, header.Get(key))
	}
	req.Header.Set(params.JujuClientVersion, jujuversion.Current.String())
	return nil
}

func isJSONMediaType(header http.Header) bool {
	return header.Get("Content-Type") == "application/json"
}

// unmarshalHTTPErrorResponse unmarshals an error response from
// an HTTP endpoint. For historical reasons, these endpoints
// return several different incompatible error response formats.
// We cope with this by accepting all of the possible formats
// and unmarshaling accordingly.
//
// It always returns a non-nil error.
func unmarshalHTTPErrorResponse(resp *http.Response) error {
	if !isJSONMediaType(resp.Header) {
		// response body is not JSON. This is probably a response
		// from the underlying webserver
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Trace(err)
		}
		switch resp.StatusCode {
		case 401:
			return errors.Unauthorizedf(string(body))
		case 403:
			return errors.Forbiddenf(string(body))
		case 404:
			return errors.NotFoundf(string(body))
		case 405:
			return errors.MethodNotAllowedf(string(body))
		default:
			return errors.New(string(body))
		}
	}

	var body json.RawMessage
	if err := httprequest.UnmarshalJSONResponse(resp, &body); err != nil {
		return errors.Trace(err)
	}
	// genericErrorResponse defines a struct that is compatible with all the
	// known error types, so that we can know which of the
	// possible error types has been returned.
	//
	// Another possible approach might be to look at resp.Request.URL.Path
	// and determine the expected error type from that, but that
	// seems more fragile than this approach.
	type genericErrorResponse struct {
		Error json.RawMessage `json:"error"`
	}
	var generic genericErrorResponse
	if err := json.Unmarshal(body, &generic); err != nil {
		return errors.Annotatef(err, "incompatible error response")
	}
	if bytes.HasPrefix(generic.Error, []byte(`"`)) {
		// The error message is in a string, which means that
		// the error must be in a params.CharmsResponse
		var resp params.CharmsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return errors.Annotatef(err, "incompatible error response")
		}
		return &params.Error{
			Message: resp.Error,
			Code:    resp.ErrorCode,
			Info:    resp.ErrorInfo,
		}
	}
	var errorBody []byte
	if len(generic.Error) > 0 {
		// We have an Error field, therefore the error must be in that.
		// (it's a params.ErrorResponse)
		errorBody = generic.Error
	} else {
		// There wasn't an Error field, so the error must be directly
		// in the body of the response.
		errorBody = body
	}
	var perr params.Error
	if err := json.Unmarshal(errorBody, &perr); err != nil {
		return errors.Annotatef(err, "incompatible error response")
	}
	if perr.Message == "" {
		return errors.Errorf("error response with no message")
	}
	return &perr
}

// bakeryError translates any discharge-required error into
// an error value that the httpbakery package will recognize.
// Other errors are returned unchanged.
func bakeryError(err error) error {
	if params.ErrCode(err) != params.CodeDischargeRequired {
		return err
	}
	errResp := errors.Cause(err).(*params.Error)
	if errResp.Info == nil {
		return errors.Annotate(err, "no error info found in discharge-required response error")
	}
	// It's a discharge-required error, so make an appropriate httpbakery
	// error from it.
	var info params.DischargeRequiredErrorInfo
	if errUnmarshal := errResp.UnmarshalInfo(&info); errUnmarshal != nil {
		return errors.Annotate(err, "unable to extract macaroon details from discharge-required response error")
	}

	bakeryErr := &httpbakery.Error{
		Message: err.Error(),
		Code:    httpbakery.ErrDischargeRequired,
		Info: &httpbakery.ErrorInfo{
			MacaroonPath: info.MacaroonPath,
		},
	}
	if info.Macaroon != nil || info.BakeryMacaroon != nil {
		// Prefer the newer bakery.v2 macaroon.
		dcMac := info.BakeryMacaroon
		if dcMac == nil {
			dcMac, err = bakery.NewLegacyMacaroon(info.Macaroon)
			if err != nil {
				return errors.Annotate(err, "unable to create legacy macaroon details from discharge-required response error")
			}
		}
		bakeryErr.Info.Macaroon = dcMac
	}
	return bakeryErr
}
