// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package miscdebug

import (
	"bufio"
	"os"
	"time"

	"microsoft.com/pkggen/internal/logger"
)

func ic(i *int) {
	*i++
}

// WaitForDebugger busy loops until manually broken out of.
// Useful for breaking in with a debugger when running privileged code.
func WaitForDebugger(tag string) {
	i := 1
	logger.Log.Errorf("Freezing at %s for debugger", tag)
	logger.Log.Errorf("Use 'break ic', then 'c' to jump to the busy loop")

	for i != 0 {
		logger.Log.Errorf("Waiting for debugger %d, once broken in run `so` to step out, `set i=0`, then `c`", i)
		ic(&i)
		time.Sleep(time.Second)
	}

}

// WaitForUser freezes until the user presses a key
func WaitForUser(tag string) {
	logger.Log.Warnf("Freezing at %s, press [ENTER] to thaw", tag)
	bufio.NewReader(os.Stdin).ReadString('\n')
	logger.Log.Warn("Thawing chroot")
}
