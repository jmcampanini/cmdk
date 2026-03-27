//go:build darwin

package trace

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func ProcessStartTime() (time.Time, error) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", os.Getpid())
	if err != nil {
		return time.Time{}, fmt.Errorf("sysctl kern.proc.pid: %w", err)
	}
	sec, nsec := kp.Proc.P_starttime.Unix()
	return time.Unix(sec, nsec), nil
}
