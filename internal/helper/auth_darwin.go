//go:build darwin

package helper

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// AllowedUIDs — UID'ы, которым разрешено слать команды helper'у.
// 0 — root, 501 — стандартный uid основного пользователя (mark).
// Права самого сокет-файла (0600, владелец root) — первый барьер; проверка UID
// отправителя через LOCAL_PEERCRED — второй, на случай если права ослаблены.
var AllowedUIDs = map[uint32]bool{
	0:   true,
	501: true,
}

// authenticatePeer проверяет UID процесса на другом конце Unix-сокета.
func authenticatePeer(conn net.Conn) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a unix socket")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return err
	}

	var (
		xucred *unix.Xucred
		sockErr error
	)
	ctrlErr := raw.Control(func(fd uintptr) {
		xucred, sockErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	})
	if ctrlErr != nil {
		return ctrlErr
	}
	if sockErr != nil {
		return sockErr
	}
	if !AllowedUIDs[xucred.Uid] {
		return fmt.Errorf("unauthorized uid: %d", xucred.Uid)
	}
	return nil
}
