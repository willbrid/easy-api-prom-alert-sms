package utils

import (
	"easy-api-prom-alert-sms/logging"

	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func GetRequestBodyFromContentType(contentType string, postParams map[string]string) (io.Reader, error) {
	var reqBody io.Reader

	switch contentType {
	case "application/x-www-form-urlencoded":
		data := url.Values{}
		for key, value := range postParams {
			data.Set(key, value)
		}
		reqBody = strings.NewReader(data.Encode())

	case "application/json":
		postParamStr, err := json.Marshal(postParams)
		if err != nil {
			return nil, err
		}
		reqBody = strings.NewReader(string(postParamStr))

	case "multipart/form-data":
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		for key, value := range postParams {
			if err := w.WriteField(key, value); err != nil {
				return nil, fmt.Errorf("failed to write field %s: %v", key, err)
			}
		}
		w.Close()
		reqBody = &b

	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	return reqBody, nil
}

// SendSMSFromApi send sms through an api specification
func SendSMSFromApi(url string, body string, authEnable bool, authType string, authCred string, timeout time.Duration, simulation bool) error {
	if simulation {
		logging.Log(logging.Info, "successful send request with url %s and body %s", url, body)
		return nil
	}

	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if authEnable {
		req.Header.Add("Authorization", fmt.Sprintf("%s %s", authType, authCred))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var respBody []byte
	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		logging.Log(logging.Error, "Failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status : %s", resp.Status)
	}

	logging.Log(logging.Info, "successful send request with url %s and body %s", url, body)
	logging.Log(logging.Info, "response body %s", string(respBody))

	return nil
}
