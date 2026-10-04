package interp

import (
	"errors"
	"net"
	"time"
)

// writeHTTP re-arms the deadline per write like SO_SNDTIMEO: a slow peer that keeps accepting bytes never times out.
func writeHTTP(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(httpRequestTimeout)); err != nil {
			return err
		}
		n, err := conn.Write(data)
		data = data[n:]
		if err != nil {
			var ne net.Error
			if n > 0 && errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return ioError(err)
		}
	}
	return nil
}
