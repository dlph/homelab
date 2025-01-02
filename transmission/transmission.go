package transmission

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"

	"github.com/hekmon/transmissionrpc/v3"
	"github.com/spf13/afero"
	"go.uber.org/zap"
)

type TransmissionClientOpt func(*TransmissionClient) error

func WithFs(fs afero.Fs) TransmissionClientOpt {
	return func(tc *TransmissionClient) error {
		tc.fs = fs
		return nil
	}
}

func WithLogger(logger *zap.Logger) TransmissionClientOpt {
	return func(tc *TransmissionClient) error {
		tc.logger = logger
		return nil
	}
}

type TransmissionClient struct {
	fs              afero.Fs
	transmissionRPC *transmissionrpc.Client
	logger          *zap.Logger
}

func NewClient(transmissionRawURL string, opts ...TransmissionClientOpt) (TransmissionClient, error) {
	client := &TransmissionClient{
		fs:     afero.NewOsFs(),
		logger: zap.L(),
	}

	endpoint, err := url.Parse(transmissionRawURL)
	if err != nil {
		return TransmissionClient{}, err
	}

	transmission, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return TransmissionClient{}, err
	}

	client.transmissionRPC = transmission

	// apply options
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return TransmissionClient{}, err
		}
	}

	return *client, nil
}

// CompletedFiles get a list of completed torrent files
// checks the fs for existence of file
// if not found on fs, it's not completed
func (cli *TransmissionClient) CompletedFiles(ctx context.Context, fs afero.Fs) ([]string, error) {
	torrents, err := cli.transmissionRPC.TorrentGetAll(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		results = slices.Grow[[]string, string](results, int(*torrent.FileCount))

		for i := 0; i < int(*torrent.FileCount); i++ {
			file := torrent.Files[i]
			fileStats := torrent.FileStats[i]

			if file.BytesCompleted != file.Length {
				continue // copy only completed files
			}

			if fileStats.BytesCompleted != file.Length {
				continue // not sure what the difference would be here
			}
			if fileStats.Wanted {
				// may want to do something with this
			}

			// get full path of torrent
			filename := filepath.Join(*torrent.DownloadDir, file.Name)

			// check the source file for existence
			srcStat, err := fs.Stat(filename)
			if err != nil {
				// file not found
				cli.logger.Warn("source file missing", zap.String("name", file.Name), zap.String("source", filename), zap.String("size", fmt.Sprintf("%d/%d", file.BytesCompleted, file.Length)))
				continue
			}

			// is the file the same size as completed torrent file
			if srcStat.Size() != file.Length {
				cli.logger.Warn("filesize mismatch", zap.String("filename", filename), zap.Int64("torrent_file_size", file.Length), zap.Int64("fs_file_size", srcStat.Size()))
				continue
			}

			// add to completed list
			results = append(results, filename)
		}
	}

	return results, nil
}

// RequestPayload recreate non-exported types from transmissionrpc package
type RequestPayload struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments,omitempty"`
	Tag       int    `json:"tag,omitempty"`
}

// AnswerPayload recreate non-exported types from transmissionrpc package
type AnswerPayload struct {
	Arguments any    `json:"arguments"`
	Result    string `json:"result"`
	Tag       *int   `json:"tag"`
}

// TorrentGetResults recreate non-exported types from transmissionrpc package
type TorrentGetResults struct {
	Torrents []transmissionrpc.Torrent `json:"torrents"`
}
