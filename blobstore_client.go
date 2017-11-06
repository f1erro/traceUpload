package apidGatewayTrace

import (
	"net/url"
	"io/ioutil"
	"encoding/json"
	"io"
	"fmt"
	"bytes"
	"net/http"
)

func (bc blobstoreClient) getSignedURL(client *http.Client, blobMetadata blobCreationMetadata, blobServerURL string) (string, error) {

	blobUri, err := url.Parse(blobServerURL)
	if err != nil {
		log.Panicf("bad url value for config %s: %s", blobUri, err)
	}

	blobUri.Path += blobStoreUri
	uri := blobUri.String()

	surl, err := bc.postWithAuth(client, uri, blobMetadata)
	if err != nil {
		log.Errorf("Unable to get signed URL from BlobServer %s: %v", uri, err)
		return "", err
	}
	defer surl.Close()

	body, err := ioutil.ReadAll(surl)
	if err != nil {
		log.Errorf("Invalid response from BlobServer for {%s} error: {%v}", uri, err)
		return "", err
	}
	res := blobServerResponse{}
	err = json.Unmarshal(body, &res)
	log.Debugf("%+v\n", res)
	if err != nil {
		log.Errorf("Invalid response from BlobServer for {%s} error: {%v}", uri, err)
		return "", err
	}


	return res.SignedUrl, nil
}

func (bc *blobstoreClient) uploadToBlobstore(client *http.Client, uriString string, data io.Reader) (*http.Response, error){
	req, err := http.NewRequest("PUT", uriString, data)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		res.Body.Close()
		return nil, fmt.Errorf("POST uri %s failed with status %d", uriString, res.StatusCode)
	}
	return res, nil
}

func (bc *blobstoreClient) postWithAuth(client *http.Client, uriString string, blobMetadata blobCreationMetadata) (io.ReadCloser, error) {

	b, err := json.Marshal(blobMetadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal blob metadata for blob %v", blobMetadata)
	}

	req, err := http.NewRequest("POST", uriString, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	// add Auth
	req.Header.Add("Authorization", getBearerToken())
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		res.Body.Close()
		return nil, fmt.Errorf("POST uri %s failed with status %d", uriString, res.StatusCode)
	}
	return res.Body, nil
}

func getBearerToken() string {
	return "Bearer " + config.GetString(configBearerToken)
}
