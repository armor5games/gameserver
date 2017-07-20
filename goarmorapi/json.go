package goarmorapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/armor5games/goarmor/goarmorconfigs"
)

type JSONRequest struct {
	Payload interface{} `json:",omitempty"`
	Time    uint64      `json:",omitempty"`
}

type JSONResponse struct {
	Success bool
	Errors  []*ErrorJSON `json:",omitempty"`
	Payload interface{}  `json:",omitempty"`
	Time    uint64       `json:",omitempty"`
}

type ErrorJSON struct {
	Code uint64
	// TODO: Rename "Error" to "Err"
	Error    error             `json:"Message,omitempty"`
	Public   bool              `json:"-"`
	Severity ErrorJSONSeverity `json:"-"`
}

type ErrorJSONSeverity uint64

const (
	ErrSeverityDebug ErrorJSONSeverity = iota
	ErrSeverityInfo
	ErrSeverityWarn
	ErrSeverityError
	ErrSeverityFatal
	ErrSeverityPanic
)

type ResponseErrorer interface {
	ResponseErrors() []*ErrorJSON
}

func (e *ErrorJSON) MarshalJSON() ([]byte, error) {
	var m string

	if e.Error != nil {
		m = e.Error.Error()
	}

	return json.Marshal(&struct {
		Code    uint64
		Message string `json:",omitempty"`
	}{
		Code:    e.Code,
		Message: m})
}

func (e *ErrorJSON) UnmarshalJSON(b []byte) error {
	s := &struct {
		Code    uint64
		Message string
	}{}

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	e.Code = s.Code

	if s.Message != "" {
		e.Error = errors.New(s.Message)
	}

	return nil
}

func (j *JSONResponse) KV() (KV, error) {
	if j == nil {
		return nil, errors.New("empty api response")
	}

	if len(j.Errors) == 0 {
		return nil, errors.New("empty key values")
	}

	kv := NewKV()

	for _, e := range j.Errors {
		if e.Code != KVAPIErrorCode {
			continue
		}

		if e.Error.Error() == "" {
			return nil, errors.New("empty kv")
		}

		x := strings.SplitN(e.Error.Error(), ":", 2)
		if len(x) != 2 {
			return nil, errors.New("bad kv format")
		}

		kv[x[0]] = x[1]
	}

	if len(kv) == 0 {
		return nil, errors.New("empty kv")
	}

	return kv, nil
}

func NewJSONRequest(
	ctx context.Context,
	responsePayload interface{}) (*JSONRequest, error) {
	return &JSONRequest{
		Payload: responsePayload,
		Time:    uint64(time.Now().Unix())}, nil
}

func NewJSONResponse(
	ctx context.Context,
	isSuccess bool,
	responsePayload interface{},
	responseErrorer ResponseErrorer,
	errs ...*ErrorJSON) (*JSONResponse, error) {
	publicErrors, err := newJSONResponseErrors(ctx, responseErrorer, errs...)
	if err != nil {
		return nil, err
	}

	return &JSONResponse{
		Success: isSuccess,
		Errors:  publicErrors,
		Payload: responsePayload,
		Time:    uint64(time.Now().Unix())}, nil
}

func newJSONResponseErrors(
	ctx context.Context,
	responseErrorer ResponseErrorer,
	errs ...*ErrorJSON) ([]*ErrorJSON, error) {
	config, ok := ctx.Value(CtxKeyConfig).(goarmorconfigs.Configer)
	if !ok {
		return nil, errors.New("context.Value fn error")
	}

	errs = append(errs, responseErrorer.ResponseErrors()...)

	var publicErrors []*ErrorJSON

	if config.ServerDebuggingLevel() > 0 {
		for _, x := range errs {
			publicErrors = append(publicErrors,
				&ErrorJSON{
					Code:     x.Code,
					Error:    errors.New(x.Error.Error()),
					Public:   x.Public,
					Severity: x.Severity})
		}

	} else {
		isKVRemoved := false

		for _, x := range errs {
			if x.Public {
				publicErrors = append(publicErrors,
					&ErrorJSON{
						Code:     x.Code,
						Error:    errors.New(x.Error.Error()),
						Public:   x.Public,
						Severity: x.Severity})

				continue
			}

			if x.Code == KVAPIErrorCode {
				isKVRemoved = true

				continue
			}

			publicErrors = append(publicErrors,
				&ErrorJSON{Code: x.Code, Severity: x.Severity})
		}

		if isKVRemoved {
			// Add empty (only with "code") "ErrorJSON" structure in order to be able to
			// determine was an key-values in hadler's response.
			publicErrors = append(publicErrors, &ErrorJSON{Code: KVAPIErrorCode})
		}
	}

	return publicErrors, nil
}
