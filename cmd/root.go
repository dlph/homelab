package cmd

import (
	"context"
	"os"

	"github.com/dlph/homelab/transmission"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/ssh"
)

const tcpNetwork = "tcp"

var rootCmd = cobra.Command{
	Use:     "run the thing",
	Short:   "",
	Long:    "",
	Example: "",
	Version: "0.0.001-alpha",
	RunE:    runRootE,
}

// TODO: log to file
func newLogger() (*zap.Logger, error) {
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel
	})

	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)

	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

	// Join the outputs, encoders, and level-handling functions into
	// zapcore.Cores, then tee the four cores together.
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleErrors, highPriority),
		zapcore.NewCore(consoleEncoder, consoleDebugging, lowPriority),
	)

	// From a zapcore.Core, it's easy to construct a Logger.
	logger := zap.New(core)

	zap.ReplaceGlobals(logger)

	return logger, nil
}

func newViperConfig(cmd *cobra.Command, fs afero.Fs, _ *zap.Logger) (*viper.Viper, error) {
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigName("config") // name of config file (without extension)
	v.SetConfigType("json")   // REQUIRED if the config file does not have the extension in the name
	v.AddConfigPath(".")
	err := v.ReadInConfig() // Find and read the config file
	if err != nil {
		return nil, err
	}

	if flag := cmd.PersistentFlags().Lookup("rules-path"); flag != nil {
		if err := v.BindPFlag("rules.paths", flag); err != nil {
			return nil, err
		}
	}

	return v, nil
}

func newSSHClient(fs afero.Fs, addr, user, privateKey string) (*ssh.Client, error) {
	// create signer from private key
	privKey, err := afero.ReadFile(fs, privateKey)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	// create config with PKI auth
	config := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial(tcpNetwork, addr, &config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func runRootE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	logger, err := newLogger()
	if err != nil {
		return err
	}

	fs := afero.NewOsFs()

	config, err := newViperConfig(cmd, fs, logger)
	if err != nil {
		return err
	}

	transmissionClient, err := transmission.NewClient(config.GetString("transmission.rpc.url"))
	if err != nil {
		return err
	}

	addr := config.GetString("transmission.sftp.addr")
	user := config.GetString("transmission.sftp.user")
	privateKey := config.GetString("transmission.sftp.private_key")
	transmissionSSHClient, err := newSSHClient(fs, addr, user, privateKey)
	if err != nil {
		logger.Error("failed to create transmission ssh client",
			zap.String("addr", addr),
			zap.String("user", user),
			zap.String("private_key", privateKey),
			zap.Error(err),
		)
		return err
	}

	sftpClient, err := sftp.NewClient(transmissionSSHClient)
	if err != nil {
		logger.Error("failed to create transmission host sftp client", zap.Error(err))
		return err
	}

	transmissionFs := sftpfs.New(sftpClient)

	files, err := transmissionClient.CompletedFiles(ctx, transmissionFs)
	if err != nil {
		logger.Error("failed to get completed torrent files from transmission", zap.Error(err))
		return err
	}
	logger.Info("completed transmission files", zap.Int("size", len(files)))

	return nil
}

func Execute() error {
	return rootCmd.Execute()
}
