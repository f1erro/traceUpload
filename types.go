package apidGatewayTrace


type createBlobMetadata struct {
	Customer    	string `json:"customer"`
	Environment 	string `json:"environment"`
	Organization 	string `json:"organization"`
	Tags 		[]string `json:"tags"`
}

type blobServerResponse struct {
	Id                       string `json:"id"`
	Kind                     string `json:"kind"`
	Self                     string `json:"self"`
	SignedUrl                string `json:"signedurl"`
	SignedUrlExpiryTimestamp string `json:"signedurlexpirytimestamp"`
}

func getTestBlobMetadata() createBlobMetadata {
	return createBlobMetadata{
		Customer: "hybrid",
		Environment: "test",
		Organization: "hybrid",
	}
}

/*
        addMutation(mw, "BlobId", blob.getId(), true, null);
        addMutation(mw, "Customer", blob.getCustomer(), true, null);
        addMutation(mw, "Environment", blob.getEnvironment(), false, null);
        addMutation(mw, "Organization", blob.getOrganization(), false, null);
        addMutation(mw, "Store", blob.getStore(), true, DEFAULT_STORE);
        addMutation(mw, "ContentType", blob.getContentType(), true, DEFAULT_CONTENT_TYPE);

        if (blob.getTags() != null) {
            mw.set("Tags").toStringArray(blob.getTags());
        } else {
            mw.set("Tags").toStringArray(Collections.EMPTY_LIST);
        }
 */