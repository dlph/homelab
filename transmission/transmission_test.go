package transmission

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hekmon/transmissionrpc/v3"
	"github.com/spf13/afero"
)

var _ afero.Fs = mockFs{}

type mockFs struct {
	statFn func(name string) (os.FileInfo, error)
}

// Chmod implements afero.Fs.
func (m mockFs) Chmod(name string, mode os.FileMode) error {
	panic("unimplemented")
}

// Chown implements afero.Fs.
func (m mockFs) Chown(name string, uid int, gid int) error {
	panic("unimplemented")
}

// Chtimes implements afero.Fs.
func (m mockFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	panic("unimplemented")
}

// Create implements afero.Fs.
func (m mockFs) Create(name string) (afero.File, error) {
	panic("unimplemented")
}

// Mkdir implements afero.Fs.
func (m mockFs) Mkdir(name string, perm os.FileMode) error {
	panic("unimplemented")
}

// MkdirAll implements afero.Fs.
func (m mockFs) MkdirAll(path string, perm os.FileMode) error {
	panic("unimplemented")
}

// Name implements afero.Fs.
func (m mockFs) Name() string {
	panic("unimplemented")
}

// Open implements afero.Fs.
func (m mockFs) Open(name string) (afero.File, error) {
	panic("unimplemented")
}

// OpenFile implements afero.Fs.
func (m mockFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	panic("unimplemented")
}

// Remove implements afero.Fs.
func (m mockFs) Remove(name string) error {
	panic("unimplemented")
}

// RemoveAll implements afero.Fs.
func (m mockFs) RemoveAll(path string) error {
	panic("unimplemented")
}

// Rename implements afero.Fs.
func (m mockFs) Rename(oldname string, newname string) error {
	panic("unimplemented")
}

// Stat implements afero.Fs.
func (m mockFs) Stat(name string) (os.FileInfo, error) {
	return m.statFn(name)
}

var _ os.FileInfo = mockFileInfo{}

type mockFileInfo struct {
	sizeFn func() int64
}

// IsDir implements fs.FileInfo.
func (m mockFileInfo) IsDir() bool {
	panic("unimplemented")
}

// ModTime implements fs.FileInfo.
func (m mockFileInfo) ModTime() time.Time {
	panic("unimplemented")
}

// Mode implements fs.FileInfo.
func (m mockFileInfo) Mode() fs.FileMode {
	panic("unimplemented")
}

// Name implements fs.FileInfo.
func (m mockFileInfo) Name() string {
	panic("unimplemented")
}

// Size implements fs.FileInfo.
func (m mockFileInfo) Size() int64 {
	return m.sizeFn()
}

// Sys implements fs.FileInfo.
func (m mockFileInfo) Sys() any {
	panic("unimplemented")
}

func ptr[T any](v T) *T {
	return &v
}

func TestCompletedFiles(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get the request tag for verificaation
		req := new(RequestPayload)
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// create a response payload
		answer := AnswerPayload{
			Arguments: TorrentGetResults{Torrents: []transmissionrpc.Torrent{
				{
					DownloadDir: ptr[string]("temp"),
					Files: []transmissionrpc.TorrentFile{
						{
							BytesCompleted: 100,
							Length:         100,
							Name:           "a",
						},
					},
					FileStats: []transmissionrpc.TorrentFileStat{
						{
							BytesCompleted: 100,
							Wanted:         true,
							Priority:       1,
						},
					},
					FileCount: ptr[int64](1),
					ID:        ptr[int64](1),
					Name:      ptr[string]("test-a"),
				},
			}},
			Result: "success",
			Tag:    &req.Tag,
		}

		if err := json.NewEncoder(w).Encode(&answer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	cli, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	fs := mockFs{
		statFn: func(name string) (fs.FileInfo, error) {
			return mockFileInfo{
				sizeFn: func() int64 {
					return int64(100)
				},
			}, nil
		},
	}

	files, err := cli.CompletedFiles(ctx, fs)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 10 {
		t.Errorf("not samezies")
	}
}
