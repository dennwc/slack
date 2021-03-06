package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var HTTPClient = &http.Client{}

type WebResponse struct {
	Ok    bool      `json:"ok"`
	Error *WebError `json:"error"`
}

type WebError string

func (s WebError) Error() string {
	return string(s)
}

func fileUploadReq(path, fieldname, filename string, values url.Values, r io.Reader) (*http.Request, error) {
	body := &bytes.Buffer{}
	wr := multipart.NewWriter(body)

	ioWriter, err := wr.CreateFormFile(fieldname, filename)
	if err != nil {
		wr.Close()
		return nil, err
	}
	_, err = io.Copy(ioWriter, r)
	if err != nil {
		wr.Close()
		return nil, err
	}
	// Close the multipart writer or the footer won't be written
	wr.Close()
	req, err := http.NewRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", wr.FormDataContentType())
	req.URL.RawQuery = (values).Encode()
	return req, nil
}

func parseResponseBody(body io.ReadCloser, intf *interface{}, debug bool) error {
	response, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	// FIXME: will be api.Debugf
	if debug {
		logger.Printf("parseResponseBody: %s\n", string(response))
	}

	err = json.Unmarshal(response, &intf)
	if err != nil {
		return err
	}

	return nil
}

func postLocalWithMultipartResponse(path, fpath, fieldname string, values url.Values, intf interface{}, debug bool) error {
	fullpath, err := filepath.Abs(fpath)
	if err != nil {
		return err
	}
	file, err := os.Open(fullpath)
	if err != nil {
		return err
	}
	defer file.Close()
	return postWithMultipartResponse(path, filepath.Base(fpath), fieldname, values, file, intf, debug)
}

func postWithMultipartResponse(path, name, fieldname string, values url.Values, r io.Reader, intf interface{}, debug bool) error {
	req, err := fileUploadReq(SLACK_API+path, fieldname, name, values, r)
	if err != nil {
		return err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Slack seems to send an HTML body along with 5xx error codes. Don't parse it.
	if resp.StatusCode != 200 {
		logResponse(resp, debug)
		return fmt.Errorf("Slack server error: %s.", resp.Status)
	}

	return parseResponseBody(resp.Body, &intf, debug)
}

func postForm(endpoint string, values url.Values, intf interface{}, debug bool) error {
	resp, err := HTTPClient.PostForm(endpoint, values)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Slack seems to send an HTML body along with 5xx error codes. Don't parse it.
	if resp.StatusCode != 200 {
		logResponse(resp, debug)
		return fmt.Errorf("Slack server error: %s.", resp.Status)
	}

	return parseResponseBody(resp.Body, &intf, debug)
}

func post(path string, values url.Values, intf interface{}, debug bool) error {
	return postForm(SLACK_API+path, values, intf, debug)
}

func parseAdminResponse(method string, teamName string, values url.Values, intf interface{}, debug bool) error {
	endpoint := fmt.Sprintf(SLACK_WEB_API_FORMAT, teamName, method, time.Now().Unix())
	return postForm(endpoint, values, intf, debug)
}

func logResponse(resp *http.Response, debug bool) error {
	if debug {
		text, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return err
		}

		logger.Print(string(text))
	}

	return nil
}
