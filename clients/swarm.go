package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
)

var (
	SwarmBaseUrl      = "http://192.168.10.77:1633"
	SwarmjsonMimeType = "application/json"
	bearerToken       string
)

type Swarm interface {
	// PinContent(ctx context.Context, name, contentType string, data io.Reader) (cid string, metadata interface{}, err error)
	Unpin(ctx context.Context, cid string) error
	UploadFile(ctx context.Context, filename, fileContentType string, data io.Reader, stampId string) (string, interface{}, error)
	CreateFeedManifest(ctl context.Context, ethcct string, swarmbatchid string) (string, error)
	//Update feed manifest (swarm-cli)
	List(ctx context.Context, pageSize, pageOffset int, cid string) (*PinList, int, error)
}

func GetBaseUrl() string {
	return SwarmBaseUrl
}

func SetBaseUrl(url string) {
	SwarmBaseUrl = url
}

func setBearerToken(token string) {
	bearerToken = token
}
func getBearerToken() string {
	return bearerToken
}

func NewSwarmClientJWT(jwt string) Swarm {
	return &SwarmClient{
		BaseClient: BaseClient{
			BaseUrl: SwarmBaseUrl,
			BaseHeaders: map[string]string{
				"Authorization": "Bearer " + jwt,
			},
		},
		bearerToken: jwt,
	}
}

func NewSwarmClientAPIKey(apiSecret, apiKey string) Swarm {
	if bearerToken == "" {
		var err error
		bearerToken, err = Authenticate(SwarmBaseUrl, apiKey, apiSecret)
		if err != nil {
			//User is not authenticated
			fmt.Println("Error authenticating user")
			return nil
		}
	}
	return &SwarmClient{
		BaseClient: BaseClient{
			BaseUrl: SwarmBaseUrl,
			BaseHeaders: map[string]string{
				"Authorization": "Bearer " + bearerToken,
			},
		},
		bearerToken: bearerToken,
	}
}

type SwarmClient struct {
	BaseClient
	Body          map[string]string
	filesMetadata []byte
	bearerToken   string // Add bearerToken field
}

type swarmuploadResponse struct {
	Reference string `json:"reference"`
}

func (p *SwarmClient) Unpin(ctx context.Context, cid string) error {
	return p.DoRequest(ctx, Request{
		Method: "DELETE",
		URL:    "/pinning/unpin/" + cid,
	}, nil)
}

func (p *SwarmClient) CreateFeedManifest(ctx context.Context, ethacct string, swarmbatchid string) (string, error) {
	//
	//
	//Prob just going to call swarm-cli here

	// return p.DoRequest(ctx, Request{
	// 	Method: "POST",
	// 	URL:    "/feeds/" + feedID,
	// 	Body:   feedManifest,
	// }, nil)
	return runSwarmCli("swarm-cli", "feed", "upload", ethacct, swarmbatchid)
}

func (p *SwarmClient) UploadFile(ctx context.Context, filename, fileContentType string, data io.Reader, stampId string) (string, interface{}, error) {
	parts := []part{
		{"file", filename, fileContentType, data},
	}
	if p.filesMetadata != nil {
		parts = append(parts)
	}
	body, contentType := multipartBody(parts)
	defer body.Close()

	var res *swarmuploadResponse
	err := p.DoRequest(ctx, Request{
		Method:      "POST",
		URL:         "/bytes",
		Headers:     map[string]string{"swarm-postage-batch-id": stampId},
		Body:        body,
		ContentType: contentType,
	}, &res)
	if err != nil {
		return "", nil, err
	}
	return res.Reference, res, nil
}

func (p *SwarmClient) List(ctx context.Context, pageSize, pageOffset int, cid string) (pl *PinList, next int, err error) {
	url := fmt.Sprintf("/data/pinList?status=pinned&pageLimit=%d&pageOffset=%d", pageSize, pageOffset)
	if cid != "" {
		url += "&hashContains=" + cid
	}
	err = p.DoRequest(ctx, Request{
		Method: "GET",
		URL:    url,
	}, &pl)
	if err != nil {
		return nil, -1, err
	}

	next = -1
	if len(pl.Pins) >= pageSize {
		next = pageOffset + len(pl.Pins)
	}
	return pl, next, err
}

func Authenticate(host, user, pass string) (string, error) {
	url := host + "/auth"
	method := "POST"

	payload := strings.NewReader(`{
			"role": "maintainer",
			"expiry": 3600
		}`)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))

	req.Header.Add("Authorization", "Basic "+encoded)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	fmt.Println(response.Key)

	return response.Key, nil
}

type Response struct {
	Key string `json:"key"`
}

func runSwarmCli(command string, args ...string) (string, error) {

	cmd := exec.Command("swarm-cli "+command, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
