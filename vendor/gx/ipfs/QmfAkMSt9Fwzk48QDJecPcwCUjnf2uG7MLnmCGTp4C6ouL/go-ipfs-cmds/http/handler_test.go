package http

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"runtime"
	"testing"

	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmfAkMSt9Fwzk48QDJecPcwCUjnf2uG7MLnmCGTp4C6ouL/go-ipfs-cmds"
)

type VersionOutput struct {
	Version string
	Commit  string
	Repo    string
	System  string
	Golang  string
}

type testEnv struct {
	version, commit, repoVersion string
	rootCtx                      context.Context
}

func (env testEnv) Context() context.Context {
	return env.rootCtx
}

func getCommit(env cmds.Environment) (string, bool) {
	tEnv, ok := env.(testEnv)
	return tEnv.commit, ok
}

func getVersion(env cmds.Environment) (string, bool) {
	tEnv, ok := env.(testEnv)
	return tEnv.version, ok
}

func getRepoVersion(env cmds.Environment) (string, bool) {
	tEnv, ok := env.(testEnv)
	return tEnv.repoVersion, ok
}

var (
	cmdRoot = &cmds.Command{
		Options: []cmdkit.Option{
			// global options, added to every command
			cmds.OptionEncodingType,
			cmds.OptionStreamChannels,
			cmds.OptionTimeout,
		},

		Subcommands: map[string]*cmds.Command{
			"version": &cmds.Command{
				Helptext: cmdkit.HelpText{
					Tagline:          "Show ipfs version information.",
					ShortDescription: "Returns the current version of ipfs and exits.",
				},
				Type: VersionOutput{},
				Options: []cmdkit.Option{
					cmdkit.BoolOption("number", "n", "Only show the version number."),
					cmdkit.BoolOption("commit", "Show the commit hash."),
					cmdkit.BoolOption("repo", "Show repo version."),
					cmdkit.BoolOption("all", "Show all version information"),
				},
				Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
					version, ok := getVersion(env)
					if !ok {
						re.SetError("couldn't get version", cmdkit.ErrNormal)
					}

					repoVersion, ok := getRepoVersion(env)
					if !ok {
						re.SetError("couldn't get repo version", cmdkit.ErrNormal)
					}

					commit, ok := getCommit(env)
					if !ok {
						re.SetError("couldn't get commit info", cmdkit.ErrNormal)
					}

					re.Emit(&VersionOutput{
						Version: version,
						Commit:  commit,
						Repo:    repoVersion,
						System:  runtime.GOARCH + "/" + runtime.GOOS, //TODO: Precise version here
						Golang:  runtime.Version(),
					})
				},
				Encoders: cmds.EncoderMap{
					cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, v *VersionOutput) error {

						if repo, ok := req.Options["repo"].(bool); ok && repo {
							_, err := fmt.Fprintf(w, "%v\n", v.Repo)
							return err
						}

						var commitTxt string
						if commit, ok := req.Options["commit"].(bool); ok && commit {
							commitTxt = "-" + v.Commit
						}

						if number, ok := req.Options["number"].(bool); ok && number {
							_, err := fmt.Fprintf(w, "%v%v\n", v.Version, commitTxt)
							return err
						}

						if all, ok := req.Options["all"].(bool); ok && all {
							_, err := fmt.Fprintf(w, "go-ipfs version: %s-%s\n"+
								"Repo version: %s\nSystem version: %s\nGolang version: %s\n",
								v.Version, v.Commit, v.Repo, v.System, v.Golang)

							return err
						}

						_, err := fmt.Fprintf(w, "ipfs version %s%s\n", v.Version, commitTxt)
						return err
					}),
				},
			},
		},
	}
)

func getTestServer(t *testing.T, origins []string) *httptest.Server {
	if len(origins) == 0 {
		origins = defaultOrigins
	}

	env := testEnv{
		version:     "0.1.2",
		commit:      "c0mm17", // yes, I know there's no 'm' in hex.
		repoVersion: "4",
		rootCtx:     context.Background(),
	}

	return httptest.NewServer(NewHandler(env, cmdRoot, originCfg(origins)))
}

func errEq(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	return err1.Error() == err2.Error()
}
