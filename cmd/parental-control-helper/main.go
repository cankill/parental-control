// Command parental-control-helper — privileged daemon (LaunchDaemon, root),
// который слушает Unix domain socket и по запросу основного приложения правит
// /etc/hosts. Вынесен в отдельный процесс, потому что основное приложение
// работает как непривилегированный LaunchAgent и не может писать в /etc/hosts.
package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"parental-control/internal/helper"
)

func main() {
	// Сокет мог остаться от прошлого запуска (демон убит без graceful shutdown).
	if err := os.Remove(helper.SockPath); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: remove stale socket: %v", err)
	}

	listener, err := net.Listen("unix", helper.SockPath)
	if err != nil {
		log.Fatalf("listen %s: %v", helper.SockPath, err)
	}

	// 0666: сокет создаёт root, а подключается непривилегированный агент (mark),
	// поэтому connect должен быть разрешён всем. Реальная защита — проверка UID
	// отправителя (LOCAL_PEERCRED, только 0/501) в helper.HandleConnection и
	// whitelist доменов. Права 0600 запрещали бы mark даже открыть сокет
	// (connect: permission denied).
	if err := os.Chmod(helper.SockPath, 0666); err != nil {
		log.Fatalf("chmod socket: %v", err)
	}

	log.Printf("parental-control-helper listening on %s", helper.SockPath)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("shutting down")
		listener.Close()
		os.Remove(helper.SockPath)
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// listener.Close() при шатдауне разблокирует Accept с ошибкой — выходим.
			log.Printf("accept: %v", err)
			return
		}
		go helper.HandleConnection(conn)
	}
}
