package gin

import (
	"strconv"
	"sync"

	"github.com/codegangsta/gin/lib"
	"github.com/go-gonzo/watch"
	"github.com/omeid/gonzo/context"
	"github.com/omeid/kargar"

	"os"
	"path/filepath"
	"time"
)

const (
	Description = `Live reload utility for Go web servers`
)

var (
	buildError error
)

type Config struct {
	Path      string // cwd.
	Port      int    //Port for the proxy server.    3000
	App       int    //Port for the Go web server.    3001
	Bin       string //Name of generated binary file. gin-bin
	Immediate bool   //Run the server immediatly after it's built" false.
}

func NewGin(config *Config, args ...string) kargar.Action {

	return func(ctx context.Context) error {
		err := config.SetDefaults()
		if err != nil {
			ctx.Fatal(err)
		}

		// Set the PORT env
		os.Setenv("PORT", strconv.Itoa(config.App))

		builder := gin.NewBuilder(config.Path, config.Bin, false)
		runner := gin.NewRunner(filepath.Join(config.Path, builder.Binary()), args...)

		runner.SetWriter(os.Stdout) //TODO: gonzo.C needs a linelogger.
		proxy := gin.NewProxy(builder, runner)

		ginconfig := &gin.Config{
			Port:    config.Port,
			ProxyTo: "http://localhost:" + strconv.Itoa(config.App),
		}

		err = proxy.Run(ginconfig)
		if err != nil {
			ctx.Fatal(err)
		}

		gin := &_gin{ctx, builder, runner, proxy, sync.Mutex{}}

		ctx.Infof("listening on port %d", config.Port)

		// build right now
		gin.Run("")

		err = watch.Watcher(ctx, gin.Run, "*.go", "*/*.go", "*/*/*.go")
		if err != nil {
			return err
		}

		<-ctx.Done()
		gin.Close()
		return nil
	}
}

type _gin struct {
	ctx     context.Context
	builder gin.Builder
	runner  gin.Runner
	proxy   *gin.Proxy
	lock    sync.Mutex
}

func (g *_gin) Run(string) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.runner.Kill()
	err := g.builder.Build()
	if err != nil {
		buildError = err
		g.ctx.Errorf("build: %s", err)
	} else {
		// print success only if there were errors before
		if buildError != nil {
			g.ctx.Info("Build Successful")
		}
		buildError = nil
	}

	time.Sleep(100 * time.Millisecond)
}

func (g *_gin) Close() {
	g.lock.Lock()

	g.ctx.Info("Got Cancel. Closing the proxy.")
	err := g.runner.Kill()
	if err != nil {
		g.ctx.Errorf("Killing: %s", err)
	}
	err = g.proxy.Close()
	if err != nil {
		g.ctx.Errorf("Stop Proxy: %s", err)
	}
}

func (c *Config) SetDefaults() error {
	if c.Port == 0 {
		c.Port = 8081
	}

	if c.App == 0 {
		c.App = 8080
	}

	if c.Bin == "" {
		c.Bin = "gin-bin"
	}

	if c.Path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		c.Path = wd
	}
	return nil
}
