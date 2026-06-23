package helper

import (
	"github.com/txn2/txeh"
)

// blockHosts добавляет домены в /etc/hosts с заворотом на BlockIP и сохраняет файл.
// Каждый вызов перечитывает /etc/hosts заново, чтобы не накапливать состояние в памяти.
func blockHosts(domains []string) error {
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		return err
	}
	hosts.AddHosts(BlockIP, domains)
	return hosts.Save()
}

// unblockHosts убирает домены из /etc/hosts и сохраняет файл.
func unblockHosts(domains []string) error {
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		return err
	}
	hosts.RemoveHosts(domains)
	return hosts.Save()
}
