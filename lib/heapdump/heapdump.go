package heapdump

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"
)

var startTime string

func init() {
	startTime = fmt.Sprintf("%d", time.Now().Unix())
}

func Write(prefix string, label string) error {
	dumpDir := filepath.Join(prefix, startTime)
	dumpFile := filepath.Join(dumpDir, label+".out")
	if err := os.MkdirAll(dumpDir, 0777); err != nil {
		return fmt.Errorf("can't make dump dir %q: %w", dumpDir, err)
	}

	f, err := os.OpenFile(dumpFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("can't make dump file %q: %w", dumpFile, err)
	}
	defer f.Close()

	return pprof.Lookup("heap").WriteTo(f, 0)
}
