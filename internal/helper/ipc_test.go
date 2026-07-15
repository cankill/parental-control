package helper

import "testing"

// validateDomains — барьер безопасности: helper от root правит /etc/hosts, поэтому
// должен блокировать ТОЛЬКО whitelisted-домены, а не произвольные (иначе клиент мог
// бы завернуть, например, apple.com или банк).
func TestValidateDomains(t *testing.T) {
	// Разрешённые из whitelist.
	if err := validateDomains([]string{"youtube.com", "www.youtube.com"}); err != nil {
		t.Errorf("allowed domains rejected: %v", err)
	}

	// Домен вне whitelist — отказ.
	if err := validateDomains([]string{"apple.com"}); err == nil {
		t.Error("apple.com should be rejected (not in whitelist)")
	}

	// Смесь разрешённого и запрещённого — отказ (нельзя пропускать частично).
	if err := validateDomains([]string{"youtube.com", "evil.com"}); err == nil {
		t.Error("mixed list with non-whitelisted domain should be rejected")
	}

	// Пустой список — отказ.
	if err := validateDomains(nil); err == nil {
		t.Error("empty domain list should be rejected")
	}
}
