// Package helper реализует privileged helper для блокировки доменов в /etc/hosts.
//
// Основное приложение (parent-control) работает как LaunchAgent от обычного
// пользователя и не имеет прав на запись в /etc/hosts. Helper (parent-control-helper)
// работает как LaunchDaemon от root, слушает Unix domain socket и по запросу
// правит /etc/hosts. Клиент и сервер общаются JSON-сообщениями через сокет.
package helper

// SockPath — путь к Unix domain socket, по которому helper принимает команды.
// Демон создаёт его с правами 0600 (владелец root), так что писать может только root.
const SockPath = "/var/run/com.simplemoves.parentcontrol.sock"

// BlockIP — адрес, на который заворачиваются заблокированные домены в /etc/hosts.
const BlockIP = "127.0.0.1"

// AllowedDomains — whitelist доменов, которые helper разрешает блокировать.
// Любой домен вне этого списка отклоняется, чтобы скомпрометированный клиент
// не мог заблокировать произвольные хосты (например, telegram или apple).
var AllowedDomains = map[string]bool{
	"youtube.com":     true,
	"www.youtube.com": true,
	"m.youtube.com":   true,
	"youtu.be":        true,
}

// Request — команда от клиента к helper.
type Request struct {
	Command string   `json:"command"` // "block" | "unblock"
	Domains []string `json:"domains"`
}

// Response — ответ helper клиенту.
type Response struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const (
	CommandBlock   = "block"
	CommandUnblock = "unblock"
)
