// Package mobile provides an entry point for gomobile to start the ondict
// HTTP server on Android.
package mobile

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/internal/httpserver"
	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/util"
)

// StartServer starts the ondict HTTP server on 127.0.0.1:<port>.
//
// configDir should be the app's private files directory
// (e.g. context.getFilesDir().getAbsolutePath() in Kotlin).
// Dictionary files (.mdx/.mdd) are expected under configDir/dicts/.
//
// cacheDir should be the app's cache directory
// (e.g. context.getCacheDir().getAbsolutePath() in Kotlin).
//
// This function blocks; call it in a goroutine from the Android Activity.
func StartServer(configDir, cacheDir string, port int) {
	// Redirect logs to a file in cacheDir so we can inspect them on device
	logFile, err := os.OpenFile(
		filepath.Join(cacheDir, "ondict.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666,
	)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.SetLevel(log.DebugLevel)

	// Catch any panic so the goroutine doesn't silently die
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("StartServer panic: %v", r)
		}
	}()

	util.SetPaths(configDir, cacheDir)
	log.Infof("StartServer: configDir=%s cacheDir=%s port=%d", configDir, cacheDir, port)

	gin.SetMode(gin.ReleaseMode)
	// dumpMDD=false: use on-demand MDD extraction via MddFileHandler instead
	sources.G.Load(true /* iexact */, false /* dumpMDD */, true /* lazy */)

	r := httpserver.New(httpserver.Options{
		History:         nil,   // no history recording on mobile
		EnableAuth:      false, // no auth on mobile
		ResourceHandler: httpserver.MddFileHandler,
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("mobile: listen %s: %v", addr, err)
	}
	log.Infof("mobile: ondict server listening on %s", addr)
	if err := r.RunListener(l); err != nil {
		log.Fatalf("mobile: server exited: %v", err)
	}
}
