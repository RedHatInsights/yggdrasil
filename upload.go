package yggdrasil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Upload constructs a multi-part MIME body composed of the contents of file and
// (optionally) metadata, creates an HTTP request, and uses the provided client
// to send the data to the platform. Upon successful upload, the request UUID is
// returned.
func Upload(client *HTTPClient, file string, collector string, metadata *CanonicalFacts) (string, error) {
	URL, err := url.Parse(client.baseURL)
	if err != nil {
		return "", err
	}
	URL.Path = path.Join(URL.Path, "/ingress/v1/upload")

	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			"file", filepath.Base(file)))
	if collector != "" {
		h.Set("Content-Type",
			fmt.Sprintf("application/vnd.redhat.%s.collection+tgz", collector))
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}
	pw, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(pw, f); err != nil {
		return "", err
	}

	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return "", err
		}

		if err := w.WriteField("metadata", string(data)); err != nil {
			return "", err
		}
	}
	w.Close()

	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, URL.String(), &buf)
	req.Header.Add("Content-Type", w.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	switch res.StatusCode {
	case http.StatusAccepted, http.StatusCreated:
		break
	case http.StatusUnsupportedMediaType:
		return "", ErrInvalidContentType
	case http.StatusRequestEntityTooLarge:
		return "", ErrPayloadTooLarge
	case http.StatusUnauthorized:
		return "", ErrUnauthorized
	default:
		return "", &APIResponseError{res.StatusCode, strings.TrimSpace(string(data))}
	}

	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return "", err
	}

	return body.RequestID, nil
}
