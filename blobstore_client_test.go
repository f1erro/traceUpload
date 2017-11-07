package apidGatewayTrace

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//"github.com/apid/apid-core/factory"
	"io/ioutil"
	"net/http/httptest"
	"net/http"
	"encoding/json"
	"io"
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
			Expect(func(){bsClient.getSignedURL(blobCreationMetadata{}, "NOT-A.UR$%L!!")}).To(Panic())
		})
	})

	Context("postWithAuth method", func() {

		It("should fetch data", func() {
			config.Set(configBearerToken, "bearer_token")
			bcm := blobCreationMetadata{
				Customer: "cust",
				Organization: "org",
				Environment: "env",
				Tags: []string{"tag1", "tag2"},

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
				Customer: "cust",
				Organization: "org",
				Environment: "env",
				Tags: []string{"tag1", "tag2"},

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

	/*Context("uploadToBlobstore", func() {
		It("should call blobstore", func() {
			bcm := blobCreationMetadata{
				Customer: "cust",
				Organization: "org",
				Environment: "env",
				Tags: []string{"tag1", "tag2"},

			}
			blobstore := httptest.NewServer(getHandler(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("PUT"))
			}))
			rc, err := bsClient.uploadToBlobstore(blobstore.URL, )
			Expect(rc).To(BeNil())
			Expect(err).ToNot(Succeed())
			blobstore.Close()
		})
	})*/

})

type blobstoreHandler struct {
	http.Handler
	handle func(http.ResponseWriter, *http.Request)
}

func (bh *blobstoreHandler) ServeHTTP(r http.ResponseWriter, req *http.Request) {
	bh.handle(r, req)
}

func getHandler(handler func(w http.ResponseWriter, r *http.Request)) http.Handler{
	return &blobstoreHandler{
		handle: handler,
	}
}

type nopCloser struct {
	io.Reader
}

//func (nopCloser) Close() os.Error { return nil }
