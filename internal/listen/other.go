//go:build !linux && !darwin && !windows

package listen

import "errors"

func list(int) ([]Socket, string, error) {
	return nil, "", errors.New("port-who does not support this OS")
}
