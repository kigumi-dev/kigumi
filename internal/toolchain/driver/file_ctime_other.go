//go:build !linux && !darwin && !freebsd

package driver

import "os"

func changeTime(os.FileInfo) int64 { return 0 }
