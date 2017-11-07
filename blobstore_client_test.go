package apidGatewayTrace

import (
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

var _ = Describe("DBManager", func() {

	var bsClient *blobstoreClient = &blobstoreClient{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: maxIdleConnsPerHost,
			},
			Timeout: httpTimeout,
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				req.Header.Set("Authorization", getBearerToken())
				return nil
			},
		},
	}
	Context("getSignedUrl method", func() {

		It("should panic with unparseable blobServerUrl", func() {
			_, err := bsClient.getSignedURL(blobCreationMetadata{}, "NOT-A.UR$%L!!")
			Expect(err).ToNot(Succeed())
			cause, ok := errors.Cause(err).(*url.Error)
			Expect(ok).To(BeTrue())
			Expect(cause).ToNot(Succeed())
		})

		It("should propagate error if error occurs during fetch of signed url", func() {
			config.Set(configBearerToken, "bearer_token")
			bcm := blobCreationMetadata{}
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
			}))
			_, err1 := bsClient.postWithAuth(blobstore.URL+blobStoreUri, bcm)
			Expect(err1).ToNot(Succeed())
			s, err2 := bsClient.getSignedURL(bcm, blobstore.URL)
			Expect(s).To(Equal(""))
			Expect(err2).ToNot(Succeed())
			//these should be the same error. This is testing proper error propagation
			Expect(err1.Error()).To(Equal(errors.Cause(err2).Error()))
			blobstore.Close()
		})

		It("should return error if blobstore returns garbage signed URL", func() {
			config.Set(configBearerToken, "bearer_token")
			bcm := blobCreationMetadata{}
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				w.Write(nil)
			}))
			s, err2 := bsClient.getSignedURL(bcm, blobstore.URL)
			Expect(s).To(Equal(""))
			Expect(err2).ToNot(Succeed())
			blobstore.Close()
		})

		It("should return signed url on success", func() {
			config.Set(configBearerToken, "bearer_token")
			bcm := blobCreationMetadata{}

			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				blobServerResponse := blobServerResponse{}
				blobServerResponse.SignedUrl = "signedurl"
				bytes, _ := json.Marshal(blobServerResponse)
				w.Write(bytes)
			}))

			s, err := bsClient.getSignedURL(bcm, blobstore.URL)
			Expect(err).To(Succeed())
			Expect(s).To(Equal("signedurl"))
			blobstore.Close()
		})
	})

	Context("postWithAuth method", func() {

		It("should fetch data", func() {
			config.Set(configBearerToken, "bearer_token")
			bcm := blobCreationMetadata{
				Customer:     "cust",
				Organization: "org",
				Environment:  "env",
				Tags:         []string{"tag1", "tag2"},
			}
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer bearer_token"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
				var recievedBcm blobCreationMetadata
				bodyBytes, err := ioutil.ReadAll(r.Body)
				Expect(err).To(Succeed())
				json.Unmarshal(bodyBytes, &recievedBcm)
				Expect(recievedBcm).To(Equal(bcm))
				w.Write([]byte("Success"))
			}))
			rc, err := bsClient.postWithAuth(blobstore.URL, bcm)
			Expect(err).To(Succeed())
			responseBytes, err := ioutil.ReadAll(rc)
			Expect(err).To(Succeed())
			Expect(responseBytes).To(Equal([]byte("Success")))
			blobstore.Close()
		})

		It("should return error if blobstore does not return 2xx", func() {
			config.Set(configBearerToken, "bearer_token")
			var handlerCalled bool
			bcm := blobCreationMetadata{
				Customer:     "cust",
				Organization: "org",
				Environment:  "env",
				Tags:         []string{"tag1", "tag2"},
			}
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(401)
			}))
			rc, err := bsClient.postWithAuth(blobstore.URL, bcm)
			Expect(rc).To(BeNil())
			Expect(err).ToNot(Succeed())
			Expect(handlerCalled).To(BeTrue())
			blobstore.Close()
		})
	})

	Context("uploadToBlobstore", func() {
		It("should call blobstore", func() {
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("PUT"))
				Expect(r.Header.Get("Content-Type"), "application/octet-stream")
				responseBytes, err := ioutil.ReadAll(r.Body)
				Expect(err).To(Succeed())
				Expect(responseBytes).To(Equal([]byte("a trace")))
				w.Write([]byte("Success"))
			}))
			content := strings.NewReader("a trace")
			r, err := bsClient.uploadToBlobstore(blobstore.URL, content)
			Expect(err).To(Succeed())
			responseBytes, err := ioutil.ReadAll(r.Body)
			Expect(err).To(Succeed())
			Expect(responseBytes).To(Equal([]byte("Success")))
			blobstore.Close()
		})

		It("should return error if storage does not return 2xx", func() {
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("PUT"))
				w.WriteHeader(401)
			}))
			content := strings.NewReader("a trace")
			r, err := bsClient.uploadToBlobstore(blobstore.URL, content)
			Expect(r).To(BeNil())
			Expect(err).ToNot(Succeed())
			blobstore.Close()
		})
	})

})

type blobstoreHandler struct {
	http.Handler
	handle func(http.ResponseWriter, *http.Request)
}

func (bh *blobstoreHandler) ServeHTTP(r http.ResponseWriter, req *http.Request) {
	bh.handle(r, req)
}

func getHandler(handler func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return &blobstoreHandler{
		handle: handler,
	}
}

type nopCloser struct {
	io.Reader
}

//func (nopCloser) Close() os.Error { return nil }
