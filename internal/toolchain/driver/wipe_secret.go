package driver

import "os"

// zeroBytes overwrites a secret's host-side Go copy; best-effort only, since
// Go's GC can leave earlier copies (growslice, GC moves) behind.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// wipeAndRemove overwrites a secret temp file with zeros before unlinking it;
// best-effort, since a COW filesystem or SSD wear leveling can outlive it.
func wipeAndRemove(path string) {
	if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
		if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
			zeros := make([]byte, fi.Size())
			f.WriteAt(zeros, 0)
			f.Sync()
			f.Close()
		}
	}
	os.Remove(path)
}
