package clients

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/golang/glog"
)

var (
	SwarmBaseUrl      = "http://192.168.10.77:1633"
	SwarmjsonMimeType = "application/json"
	bearerToken       string
)

type Swarm interface {
	// PinContent(ctx context.Context, name, contentType string, data io.Reader) (cid string, metadata interface{}, err error)
	Unpin(ctx context.Context, cid string) error
	UploadFile(ctx context.Context, filename, fileContentType, stampId string, data io.Reader) (string, error)
	UploadFeedManifest(ctx context.Context, topic string, filename string, fileContentType string, stampId string, data io.Reader) (string, error)
	//Update feed manifest (swarm-cli)
	// List(ctx context.Context, pageSize, pageOffset int, cid string) (*PinList, int, error)
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

// type SwarmClient struct {
// 	BaseClient
// 	Body          map[string]string
// 	filesMetadata []byte
// 	bearerToken   string // Add bearerToken field
// }

// func (p *SwarmClient) Unpin(ctx context.Context, cid string) error {
// 	return p.DoRequest(ctx, Request{
// 		Method: "DELETE",
// 		URL:    "/pinning/unpin/" + cid,
// 	}, nil)
// }

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

func (p *SwarmClient) UploadFeedManifest(ctx context.Context, topic string, filename string, fileContentType string, stampId string, data io.Reader) (string, error) {
	byteArray, err := ioutil.ReadAll(data)
	if err != nil {
		errtxt := "error reading byte data, aborting manifest upload: "
		glog.Error(errtxt, err)
		return errtxt, err
	} else {
		output, err := runSwarmCliWithStdin(byteArray, "feed", "upload", "--topic-string", strings.ReplaceAll(topic, "/", "_"), "--stamp", stampId, "--name", strings.ReplaceAll(filename, "/", "_"), "--content-type", fileContentType, "--bee-api-url", p.BaseUrl, "--bee-debug-api-url", p.BaseUrl, "-H", "Authorization: Bearer "+p.bearerToken, "--identity", "main", "--password", "1234", "--stdin")
		if err != nil {
			glog.Error("error uploading file to swarm: ", err)
			return "", err
		} else {
			SwarmUrl := ParseFeedManifestURL(string(output))
			glog.Infof("uploaded manifest file to swarm: reference: %s filename: %s", SwarmUrl, filename)
			// glog.Infof("Swarm hash: %s", SwarmUrl)
			return SwarmUrl, nil
		}
	}
}

// UploadFile uploads a file to Swarm and returns the reference id to the file
func (p *SwarmClient) UploadFile(ctx context.Context, filename, fileContentType, stampId string, data io.Reader) (string, error) {
	byteArray, err := ioutil.ReadAll(data)
	if err != nil {
		errtxt := "error reading byte data, aborting manifest upload: "
		glog.Error(errtxt, err)
		return errtxt, err
	} else {
		output, err := runSwarmCliWithStdin(byteArray, "upload", "--stamp", stampId, "--name", filename, "--content-type", fileContentType, "--bee-api-url", p.BaseUrl, "--bee-debug-api-url", p.BaseUrl, "-H", "Authorization: Bearer "+p.bearerToken, "--stdin")

		if err != nil {
			glog.Error("error uploading file to swarm: ", err)
			return "", err
		} else {
			SwarmUrl := ParseURL(string(output))
			glog.Infof("uploaded file to swarm: reference: %s filename: %s", SwarmUrl, filename)
			return SwarmUrl, nil
		}
	}
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

	glog.Info("Successfully obtained bearerToken for bee admin")
	return response.Key, nil
}

type Response struct {
	Key string `json:"key"`
}

func runSwarmCliWithStdin(stdinData []byte, args ...string) (string, error) {
	// glog.Info("running swarm-cli with stdin: " + strings.Join(args, " "))
	cmd := exec.Command("swarm-cli", args...)
	cmd.Stdin = bytes.NewReader(stdinData)
	output, err := cmd.CombinedOutput()
	if err != nil {
		glog.Error(err)
		return "", err
	} else {
		fmt.Println(string(output))
		return strings.TrimSpace(string(output)), nil
	}

}

func ParseFeedManifestURL(data string) string {
	re := regexp.MustCompile(`Feed Manifest URL: (http:\/\/\S+)`)
	match := re.FindStringSubmatch(data)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func ParseURL(data string) string {
	re := regexp.MustCompile(`URL: (http:\/\/\S+)`)
	match := re.FindStringSubmatch(data)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func ParseSwarmHash(data string) string {
	re := regexp.MustCompile(`Swarm hash: (\S+)`)
	match := re.FindStringSubmatch(data)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// func runSwarmCli(command string, args ...string) (string, error) {

// 	cmd := exec.Command("swarm-cli "+command, args...)
// 	output, err := cmd.Output()
// 	if err != nil {
// 		return "", err
// 	}
// 	return strings.TrimSpace(string(output)), nil
// }
