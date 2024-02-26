package drivers

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-tools/clients"
)

type SwarmOS struct {
	key       string
	secret    string
	hostname  string
	stampinfo stampInfo
}

// Stamp will be used to upload new video segments
// FeedStamp will contain the latest playlist of video segments
type stampInfo struct {
	Stamp     string `json:"stamp"`
	FeedStamp string `json:"feedstamp"`
}

var _ OSSession = (*SwarmSession)(nil)

type SwarmSession struct {
	os       *SwarmOS
	filename string
	client   clients.Swarm
	dCache   map[string]*dataCache
	dLock    sync.RWMutex
}

func NewSwarmDriver(hostname, key, secret string, stampinfo stampInfo) *SwarmOS {
	return &SwarmOS{hostname: hostname, key: key, secret: secret, stampinfo: stampinfo}
}

func (ostore *SwarmOS) NewSession(filename string) OSSession {
	var client clients.Swarm
	if ostore.key != "" {
		client = clients.NewSwarmClientAPIKey(ostore.key, ostore.secret)
		ostore.key = ""
	} else {
		client = clients.NewSwarmClientJWT(ostore.secret)
	}
	session := &SwarmSession{
		os:       ostore,
		filename: filename,
		dCache:   make(map[string]*dataCache),
		dLock:    sync.RWMutex{},
		client:   client,
	}

	return session
}

func (ostore *SwarmOS) UriSchemes() []string {
	return []string{"bzz://" + ostore.hostname}
}

func (ostore *SwarmOS) Description() string {
	return "Swarm CLI Go Driver"
}

func (ostore *SwarmOS) Publish(ctx context.Context) (string, error) {
	return "", ErrNotSupported
}

func (session *SwarmSession) OS() OSDriver {
	return session.os
}

func (session *SwarmSession) EndSession() {
	// no op
}

func (session *SwarmSession) ListFiles(ctx context.Context, cid, delim string) (PageInfo, error) {
	return nil, ErrNotSupported

	// pinList, _, err := session.client.List(ctx, 1, 0, cid)
	// pi := &singlePageInfo{
	// 	files:       []FileInfo{},
	// 	directories: []string{},
	// }
	// if err == nil && pinList.Count == 1 {
	// 	size := pinList.Pins[0].Size
	// 	pi.files = append(pi.files, FileInfo{Name: pinList.Pins[0].Metadata.Name, Size: &size,
	// 		ETag: pinList.Pins[0].IPFSPinHash})
	// }
	// return pi, err
}

func CreateFeedManifest(ctl context.Context, ethacct string, swarmbatchid string) (string, error) {
	return runSwarmCli("feed", "create", ethacct, swarmbatchid)
}

func (session *SwarmSession) ReadData(ctx context.Context, referenceid string) (*FileInfoReader, error) {
	fullPath := path.Join(session.filename, referenceid)
	resp, err := http.Get("https://" + session.os.hostname + "/bytes" + fullPath)
	if err != nil {
		return nil, err
	}
	res := &FileInfoReader{
		FileInfo: FileInfo{
			Name: session.filename,
			Size: nil,
		},
		Body: resp.Body,
	}
	return res, nil
}

func (session *SwarmSession) ReadDataRange(ctx context.Context, name, byteRange string) (*FileInfoReader, error) {
	return nil, ErrNotSupported
}

func (session *SwarmSession) Presign(name string, expire time.Duration) (string, error) {
	return "", ErrNotSupported
}

func (session *SwarmSession) IsExternal() bool {
	return false
}

func (session *SwarmSession) IsOwn(url string) bool {
	return true
}

func (session *SwarmSession) GetInfo() *OSInfo {
	return nil
}

func (ostore *SwarmSession) DeleteFile(ctx context.Context, name string) error {
	return ErrNotSupported
}

// What does it mean to save data to swarm? We are not pinning it.
func (session *SwarmSession) SaveData(ctx context.Context, name string, data io.Reader, fields *FileProperties, timeout time.Duration) (*SaveDataOutput, error) {
	// concatenate filename with name argument to get full filename, both may be empty
	fullPath := session.getAbsolutePath(name)
	if fullPath == "" {
		// pinata requires name to be set
		fullPath = "data.bin"
	}

	if strings.Contains(fullPath, ".m3u8") {
		//Send it to the swarm-cli

		//Does a manifest exist for this root hash reference id?
		cid, _ := session.client.CreateFeedManifest(ctx, "", session.os.stampinfo.FeedStamp)

		if cid == "" {
			//just  a stub
		}
		// fullPath
	}

	cid, _, err := session.client.UploadFile(ctx, fullPath, "", data, session.os.stampinfo.Stamp)

	return &SaveDataOutput{URL: cid}, err
}

func (session *SwarmSession) getAbsolutePath(name string) string {
	resPath := path.Clean(session.filename + "/" + name)
	if resPath == "/" {
		return ""
	}
	return resPath
}

func runSwarmCli(command string, args ...string) (string, error) {

	cmd := exec.Command("swarm-cli "+command, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
