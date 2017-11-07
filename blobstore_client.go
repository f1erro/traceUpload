package apidGatewayTrace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

func (bc *blobstoreClient) getSignedURL(blobMetadata blobCreationMetadata, blobServerURL string) (string, error) {

	blobUri, err := url.Parse(blobServerURL)
	if err != nil {
		//do not panic here, apid should live even if trace plugin was misconfigured
		return "", errors.Wrapf(err, "bad url value for config %s: %s", blobUri, err)
	}

	blobUri.Path += blobStoreUri
	uri := blobUri.String()

	surl, err := bc.postWithAuth(uri, blobMetadata)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to get signed URL from BlobServer %s: %v", uri, err)
	}
	defer surl.Close()

	body, err := ioutil.ReadAll(surl)
	if err != nil {
		return "", errors.Wrapf(err, "Invalid response from BlobServer for {%s} error: {%v}", uri, err)
	}
	res := blobServerResponse{}
	err = json.Unmarshal(body, &res)
	log.Debugf("%+v\n", res)
	if err != nil {
		return "", errors.Wrapf(err, "Invalid response from BlobServer for {%s} error: {%v}", uri, err)
	}

	return res.SignedUrl, nil
}

func (bc *blobstoreClient) uploadToBlobstore(uriString string, data io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("PUT", uriString, data)
	if err != nil {
		return nil, errors.Wrap(err, "error in returned by http.NewRequest")
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	res, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http error in attempt to upload to blobstore")
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		res.Body.Close()
		return nil, errors.New(fmt.Sprintf("POST uri %s failed with status %d", uriString, res.StatusCode))
	}
	return res, nil
}

func (bc *blobstoreClient) postWithAuth(uriString string, blobMetadata blobCreationMetadata) (io.ReadCloser, error) {

	b, err := json.Marshal(blobMetadata)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to marshal blob metadata for blob %v", blobMetadata)
	}

	req, err := http.NewRequest("POST", uriString, bytes.NewReader(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new request via call to http.NewRequest")
	}
	// add Auth
	req.Header.Add("Authorization", getBearerToken())
	req.Header.Add("Content-Type", "application/json")
	res, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error in attempt to POST to blobstore")
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		res.Body.Close()
		return nil, errors.New(fmt.Sprintf("POST uri %s failed with status %d", uriString, res.StatusCode))
	}
	return res.Body, nil
}

func getBearerToken() string {
	return "Bearer " + config.GetString(configBearerToken)
}
